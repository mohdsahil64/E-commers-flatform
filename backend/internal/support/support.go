// Package support implements the Help & Support ticketing flow:
//
//	customer opens a ticket from the Help Centre, choosing a category.
//	  * product issue       -> routed to the SELLER of the chosen order item
//	  * payment/login/other -> routed to ADMIN
//
// The SELLER replies to and resolves only its own product tickets. ADMIN
// replies to and resolves every non-product ticket, and may READ (but never
// act on) product tickets so it can audit the customer<->seller conversation.
//
// Every reply by any party is a row in support_ticket_messages, so all three
// portals render one shared, ordered timeline. Photo proof the customer
// attaches at open time lives in support_ticket_photos (stored via the shared
// upload endpoint → Cloudinary).
package support

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// Ticket status values.
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusSolved     = "solved"
)

// Assignee values — who owns (handles) the ticket.
const (
	AssigneeSeller = "seller"
	AssigneeAdmin  = "admin"
)

// productCategory is the only category that routes to a seller. Everything else
// routes to admin.
const productCategory = "product"

// validCategories bounds what a customer may submit.
var validCategories = map[string]bool{
	"product": true,
	"payment": true,
	"login":   true,
	"account": true,
	"refund":  true,
	"other":   true,
}

// Message is one entry in a ticket's conversation timeline.
type Message struct {
	ID         string    `json:"id"`
	AuthorRole string    `json:"author_role"` // customer | seller | admin | system
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// Photo is a customer-uploaded proof image (Cloudinary key).
type Photo struct {
	FileKey string `json:"file_key"`
}

// Ticket is the rich view model shared across portals.
type Ticket struct {
	ID           string `json:"id"`
	TicketNumber string `json:"ticket_number"`
	CustomerID   string `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	Category     string `json:"category"`
	Assignee     string `json:"assignee"`
	SellerID     string `json:"seller_id"`
	SellerName   string `json:"seller_name"`
	// Product context (product tickets only).
	OrderID      string `json:"order_id"`
	OrderNumber  string `json:"order_number"`
	OrderItemID  string `json:"order_item_id"`
	ProductTitle string `json:"product_title"`
	ProductImage string `json:"product_image"`
	VariantLabel string `json:"variant_label"`

	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Resolution  string    `json:"resolution"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Photos   []Photo   `json:"photos"`
	Messages []Message `json:"messages"`
}

// Repository provides ticket persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a support Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool exposes the underlying pool.
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// NotifyHook fires after a ticket is created or resolved so other modules
// (email) can react without this package importing them.
//   - event "created"  → send the "we received your request" email
//   - event "resolved" → send the "here's what happened / resolution" email
type NotifyHook func(ctx context.Context, ticketID, event string)

// Service holds support business logic.
type Service struct {
	repo   *Repository
	notify NotifyHook
}

// NewService builds a support Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo, notify: func(context.Context, string, string) {}}
}

// WithNotifier attaches the create/resolve notification hook.
func (s *Service) WithNotifier(h NotifyHook) *Service {
	if h != nil {
		s.notify = h
	}
	return s
}

// OpenInput is the customer "open a ticket" payload.
type OpenInput struct {
	UserID      string
	Category    string
	Subject     string
	Description string
	OrderItemID string   // required when Category == "product"
	PhotoKeys   []string
}

// Open creates a ticket for the requesting customer and routes it. Product
// tickets go to the seller of the chosen order item (which must belong to the
// customer); everything else goes to admin.
func (s *Service) Open(ctx context.Context, in OpenInput) (string, error) {
	in.Category = strings.ToLower(strings.TrimSpace(in.Category))
	if !validCategories[in.Category] {
		return "", httpx.BadRequest("validation_failed", "Choose a valid help category")
	}
	if strings.TrimSpace(in.Description) == "" {
		return "", httpx.BadRequest("validation_failed", "Please describe your issue")
	}

	var ticketID string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		// Resolve the customer id from the user id.
		var customerID string
		if err := tx.QueryRow(ctx,
			`SELECT id FROM customers WHERE user_id = $1`, in.UserID).Scan(&customerID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.Forbidden("no_customer", "Customer profile not found")
			}
			return err
		}

		assignee := AssigneeAdmin
		var sellerID, orderID, orderItemID *string

		if in.Category == productCategory {
			if in.OrderItemID == "" {
				return httpx.BadRequest("validation_failed", "Select the product you need help with")
			}
			// The order item must belong to this customer. Capture its seller
			// and order so the ticket is routed + contextualised.
			var sID, oID string
			err := tx.QueryRow(ctx, `
				SELECT oi.seller_id, oi.order_id
				FROM order_items oi
				JOIN orders o ON o.id = oi.order_id
				WHERE oi.id = $1 AND o.customer_id = $2`, in.OrderItemID, customerID).
				Scan(&sID, &oID)
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.Forbidden("not_eligible", "You can only raise product issues for items you ordered")
			}
			if err != nil {
				return err
			}
			assignee = AssigneeSeller
			sellerID, orderID, orderItemID = &sID, &oID, &in.OrderItemID
		}

		subject := strings.TrimSpace(in.Subject)
		if subject == "" {
			subject = defaultSubject(in.Category)
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO support_tickets
			  (ticket_number, customer_id, category, assignee, seller_id,
			   order_id, order_item_id, subject, description, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'open')
			RETURNING id`,
			ticketNumber(), customerID, in.Category, assignee, sellerID,
			orderID, orderItemID, subject, strings.TrimSpace(in.Description)).
			Scan(&ticketID); err != nil {
			return err
		}

		// The opening description becomes the first timeline message (customer).
		if _, err := tx.Exec(ctx, `
			INSERT INTO support_ticket_messages (ticket_id, author_role, author_id, body)
			VALUES ($1,'customer',$2,$3)`, ticketID, in.UserID, strings.TrimSpace(in.Description)); err != nil {
			return err
		}

		for i, key := range in.PhotoKeys {
			if key == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO support_ticket_photos (ticket_id, file_key, sort_order) VALUES ($1,$2,$3)`,
				ticketID, key, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil && ticketID != "" {
		s.notify(context.Background(), ticketID, "created")
	}
	return ticketID, err
}

// ReplyInput adds a message to a ticket's timeline.
type ReplyInput struct {
	TicketID   string
	UserID     string
	AuthorRole string // customer | seller | admin
	Body       string
	// Authorisation scope: for a seller reply, SellerID must own the ticket;
	// for a customer reply, the ticket must belong to the customer.
	SellerID   string
}

// Reply appends a message after verifying the author is allowed to act on the
// ticket. A solved ticket cannot receive new replies.
func (s *Service) Reply(ctx context.Context, in ReplyInput) error {
	if strings.TrimSpace(in.Body) == "" {
		return httpx.BadRequest("validation_failed", "Message cannot be empty")
	}
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		t, err := lockTicket(ctx, tx, in.TicketID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, tx, t, in.AuthorRole, in.UserID, in.SellerID); err != nil {
			return err
		}
		if t.status == StatusSolved {
			return httpx.Conflict("ticket_solved", "This ticket is already solved")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO support_ticket_messages (ticket_id, author_role, author_id, body)
			VALUES ($1,$2,$3,$4)`, in.TicketID, in.AuthorRole, in.UserID, strings.TrimSpace(in.Body)); err != nil {
			return err
		}
		// A responder's first reply moves an open ticket to in_progress.
		if t.status == StatusOpen && in.AuthorRole != "customer" {
			if _, err := tx.Exec(ctx,
				`UPDATE support_tickets SET status='in_progress', updated_at=now() WHERE id=$1`, in.TicketID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx,
				`UPDATE support_tickets SET updated_at=now() WHERE id=$1`, in.TicketID); err != nil {
				return err
			}
		}
		return nil
	})
}

// ResolveInput marks a ticket solved with a resolution note (the responder's
// "what happened / what we did" text surfaced in the resolution email).
type ResolveInput struct {
	TicketID   string
	UserID     string
	AuthorRole string // seller | admin
	Resolution string
	SellerID   string
}

// Resolve marks a ticket solved. Only the ticket's responder may resolve it:
// the owning seller for product tickets, admin for the rest.
func (s *Service) Resolve(ctx context.Context, in ResolveInput) error {
	if strings.TrimSpace(in.Resolution) == "" {
		return httpx.BadRequest("validation_failed", "Add a short resolution note before marking solved")
	}
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		t, err := lockTicket(ctx, tx, in.TicketID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, tx, t, in.AuthorRole, in.UserID, in.SellerID); err != nil {
			return err
		}
		if in.AuthorRole == "customer" {
			return httpx.Forbidden("not_allowed", "Only the support team can mark a ticket solved")
		}
		if t.status == StatusSolved {
			return httpx.Conflict("ticket_solved", "This ticket is already solved")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE support_tickets
			SET status='solved', resolution=$2, resolved_by=$3, resolved_at=now(), updated_at=now()
			WHERE id=$1`, in.TicketID, strings.TrimSpace(in.Resolution), in.UserID); err != nil {
			return err
		}
		// Record the resolution as a closing system/responder message so the
		// timeline shows exactly what was said.
		_, err = tx.Exec(ctx, `
			INSERT INTO support_ticket_messages (ticket_id, author_role, author_id, body)
			VALUES ($1,$2,$3,$4)`, in.TicketID, in.AuthorRole, in.UserID,
			"Marked solved: "+strings.TrimSpace(in.Resolution))
		return err
	})
	if err == nil {
		s.notify(context.Background(), in.TicketID, "resolved")
	}
	return err
}

// lockedTicket is the minimal row used for authorisation during a transition.
type lockedTicket struct {
	id         string
	assignee   string
	sellerID   string
	customerID string
	status     string
}

func lockTicket(ctx context.Context, tx pgx.Tx, ticketID string) (lockedTicket, error) {
	var t lockedTicket
	var sellerID, customerID *string
	err := tx.QueryRow(ctx, `
		SELECT id, assignee, seller_id, customer_id, status
		FROM support_tickets WHERE id=$1 FOR UPDATE`, ticketID).
		Scan(&t.id, &t.assignee, &sellerID, &customerID, &t.status)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, httpx.NotFound("ticket_not_found", "Support ticket not found")
	}
	if err != nil {
		return t, err
	}
	if sellerID != nil {
		t.sellerID = *sellerID
	}
	if customerID != nil {
		t.customerID = *customerID
	}
	return t, nil
}

// authorize checks that the actor may act on the ticket.
//   - customer: must own the ticket.
//   - seller:   ticket must be assigned to the seller AND be this seller's.
//   - admin:    may act only on admin-assigned tickets (product tickets are
//     seller-owned and admin is read-only over them).
func (s *Service) authorize(ctx context.Context, tx pgx.Tx, t lockedTicket, role, userID, sellerID string) error {
	switch role {
	case "customer":
		var ok bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1 AND user_id=$2)`,
			t.customerID, userID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return httpx.Forbidden("not_your_ticket", "This ticket does not belong to you")
		}
	case "seller":
		if t.assignee != AssigneeSeller || t.sellerID == "" || t.sellerID != sellerID {
			return httpx.Forbidden("not_your_ticket", "This ticket is not assigned to your store")
		}
	case "admin":
		if t.assignee != AssigneeAdmin {
			return httpx.Forbidden("read_only", "Product tickets are handled by the seller; admin access is read-only")
		}
	default:
		return httpx.Forbidden("not_allowed", "You cannot act on this ticket")
	}
	return nil
}

// ---- listing / detail ----

// ListForCustomer returns a customer's tickets (newest first).
func (s *Service) ListForCustomer(ctx context.Context, userID string) ([]Ticket, error) {
	return s.list(ctx, `WHERE t.customer_id = (SELECT id FROM customers WHERE user_id = $1)`, userID)
}

// ListForSeller returns tickets assigned to a seller.
func (s *Service) ListForSeller(ctx context.Context, sellerID string) ([]Ticket, error) {
	return s.list(ctx, `WHERE t.assignee = 'seller' AND t.seller_id = $1`, sellerID)
}

// ListForAdmin returns tickets for the admin desk. By default admin sees the
// tickets it owns (assignee='admin'); scope="all" additionally surfaces the
// read-only product (seller) tickets so admin can audit the conversation.
func (s *Service) ListForAdmin(ctx context.Context, scope, status string) ([]Ticket, error) {
	where := `WHERE t.assignee = 'admin'`
	if scope == "all" {
		where = `WHERE TRUE`
	}
	args := []any{}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(` AND t.status = $%d`, len(args))
	}
	return s.list(ctx, where, args...)
}

func (s *Service) list(ctx context.Context, where string, args ...any) ([]Ticket, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT t.id, t.ticket_number, t.customer_id, coalesce(cu.display_name,''),
		       t.category, t.assignee, coalesce(t.seller_id::text,''), coalesce(se.store_name,''),
		       coalesce(t.order_id::text,''), coalesce(o.order_number,''),
		       coalesce(t.order_item_id::text,''), coalesce(oi.product_title,''),
		       coalesce((SELECT file_key FROM product_images pi
		                 JOIN seller_listings sl ON sl.product_id = pi.product_id
		                 WHERE sl.id = oi.seller_listing_id
		                 ORDER BY pi.is_primary DESC, pi.sort_order LIMIT 1),''),
		       coalesce(oi.variant_label,''),
		       t.subject, t.description, t.status, t.resolution, t.resolved_at,
		       t.created_at, t.updated_at
		FROM support_tickets t
		LEFT JOIN customers cu ON cu.id = t.customer_id
		LEFT JOIN sellers   se ON se.id = t.seller_id
		LEFT JOIN orders    o  ON o.id  = t.order_id
		LEFT JOIN order_items oi ON oi.id = t.order_item_id
		`+where+` ORDER BY t.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ticket
	ids := []string{}
	index := map[string]int{}
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.TicketNumber, &t.CustomerID, &t.CustomerName,
			&t.Category, &t.Assignee, &t.SellerID, &t.SellerName,
			&t.OrderID, &t.OrderNumber, &t.OrderItemID, &t.ProductTitle,
			&t.ProductImage, &t.VariantLabel,
			&t.Subject, &t.Description, &t.Status, &t.Resolution, &t.ResolvedAt,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Photos = []Photo{}
		t.Messages = []Message{}
		index[t.ID] = len(out)
		out = append(out, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	if err := s.attachPhotos(ctx, out, index, ids); err != nil {
		return nil, err
	}
	if err := s.attachMessages(ctx, out, index, ids); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) attachPhotos(ctx context.Context, out []Ticket, index map[string]int, ids []string) error {
	rows, err := s.repo.pool.Query(ctx,
		`SELECT ticket_id, file_key FROM support_ticket_photos WHERE ticket_id = ANY($1) ORDER BY sort_order`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tid string
		var p Photo
		if err := rows.Scan(&tid, &p.FileKey); err != nil {
			return err
		}
		if i, ok := index[tid]; ok {
			out[i].Photos = append(out[i].Photos, p)
		}
	}
	return rows.Err()
}

func (s *Service) attachMessages(ctx context.Context, out []Ticket, index map[string]int, ids []string) error {
	rows, err := s.repo.pool.Query(ctx,
		`SELECT ticket_id, id, author_role, body, created_at
		 FROM support_ticket_messages WHERE ticket_id = ANY($1) ORDER BY created_at`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tid string
		var m Message
		if err := rows.Scan(&tid, &m.ID, &m.AuthorRole, &m.Body, &m.CreatedAt); err != nil {
			return err
		}
		if i, ok := index[tid]; ok {
			out[i].Messages = append(out[i].Messages, m)
		}
	}
	return rows.Err()
}

func defaultSubject(category string) string {
	switch category {
	case "product":
		return "Product issue"
	case "payment":
		return "Payment issue"
	case "login":
		return "Login / account access"
	case "account":
		return "Account issue"
	case "refund":
		return "Refund query"
	default:
		return "Help request"
	}
}

func ticketNumber() string {
	return fmt.Sprintf("HLP-%d", time.Now().UnixNano()%1_000_000_000)
}
