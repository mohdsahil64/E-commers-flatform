package support

// Seller -> Admin support. A seller raises a request to the CARLITOS admin
// team and chats with them live. The seller may attach proof photos; admin
// can only view them. Admin replies and marks the request solved.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/httpx"
)

// scopeSellerSupport tags real-time events that belong to seller -> admin tickets.
const scopeSellerSupport = "seller_support"

// validSellerCategories bounds what a seller may submit.
var validSellerCategories = map[string]bool{
	"payments":  true, // settlements, payouts, commission
	"orders":    true, // orders, shipping, returns
	"products":  true, // listing, catalog, approval
	"account":   true, // profile, KYC, bank details
	"technical": true, // portal not working, bugs
	"other":     true,
}

// SellerTicket is the view model for a seller -> admin ticket.
type SellerTicket struct {
	ID           string     `json:"id"`
	TicketNumber string     `json:"ticket_number"`
	SellerID     string     `json:"seller_id"`
	SellerName   string     `json:"seller_name"`
	Category     string     `json:"category"`
	Subject      string     `json:"subject"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	Resolution   string     `json:"resolution"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Photos       []Photo    `json:"photos"`
	Messages     []Message  `json:"messages"`
}

// SellerOpenInput is the seller "raise a request" payload.
type SellerOpenInput struct {
	SellerID    string
	UserID      string
	Category    string
	Subject     string
	Description string
	PhotoKeys   []string
}

// OpenSellerTicket creates a seller -> admin ticket.
func (s *Service) OpenSellerTicket(ctx context.Context, in SellerOpenInput) (string, error) {
	in.Category = strings.ToLower(strings.TrimSpace(in.Category))
	if !validSellerCategories[in.Category] {
		return "", httpx.BadRequest("validation_failed", "Choose a valid help category")
	}
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		return "", httpx.BadRequest("validation_failed", "Please describe your issue")
	}
	if len(in.PhotoKeys) > 4 {
		return "", httpx.BadRequest("validation_failed", "You can attach up to 4 photos")
	}
	for _, k := range in.PhotoKeys {
		if !validImageKey(k) {
			return "", httpx.BadRequest("validation_failed", "Invalid image")
		}
	}
	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		subject = defaultSellerSubject(in.Category)
	}

	var ticketID string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO seller_support_tickets
			  (ticket_number, seller_id, category, subject, description, status)
			VALUES ($1,$2,$3,$4,$5,'open') RETURNING id`,
			sellerTicketNumber(), in.SellerID, in.Category, subject, desc).Scan(&ticketID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_support_messages (ticket_id, author_role, author_id, body)
			VALUES ($1,'seller',$2,$3)`, ticketID, in.UserID, desc); err != nil {
			return err
		}
		for i, k := range in.PhotoKeys {
			if k == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO seller_support_photos (ticket_id, file_key, sort_order) VALUES ($1,$2,$3)`,
				ticketID, k, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		s.publishSellerChanged(ticketID, in.SellerID, StatusOpen)
	}
	return ticketID, err
}

// SellerReplyInput adds a message to a seller -> admin ticket.
type SellerReplyInput struct {
	TicketID   string
	UserID     string
	AuthorRole string // seller | admin
	SellerID   string // required for AuthorRole == seller (ownership check)
	Body       string
	ImageKey   string
}

// ReplySellerTicket appends a message and pushes it live. Photos are
// seller-only; a solved ticket accepts no further messages.
func (s *Service) ReplySellerTicket(ctx context.Context, in SellerReplyInput) (Message, error) {
	in.Body = strings.TrimSpace(in.Body)
	in.ImageKey = strings.TrimSpace(in.ImageKey)
	if in.Body == "" && in.ImageKey == "" {
		return Message{}, httpx.BadRequest("validation_failed", "Message cannot be empty")
	}
	if !validImageKey(in.ImageKey) {
		return Message{}, httpx.BadRequest("validation_failed", "Invalid image")
	}
	if in.ImageKey != "" && in.AuthorRole != "seller" {
		return Message{}, httpx.Forbidden("not_allowed", "Only the seller can send photos")
	}

	if _, err := uuid.Parse(in.TicketID); err != nil {
		return Message{}, httpx.NotFound("ticket_not_found", "Support ticket not found")
	}

	var msg Message
	var ticketSellerID, newStatus string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx,
			`SELECT seller_id::text, status FROM seller_support_tickets WHERE id=$1 FOR UPDATE`, in.TicketID).
			Scan(&ticketSellerID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("ticket_not_found", "Support ticket not found")
		}
		if err != nil {
			return err
		}
		if in.AuthorRole == "seller" && ticketSellerID != in.SellerID {
			return httpx.Forbidden("not_your_ticket", "This ticket does not belong to your store")
		}
		if status == StatusSolved {
			return httpx.Conflict("ticket_solved", "This ticket is already solved")
		}
		msg = Message{AuthorRole: in.AuthorRole, Body: in.Body, ImageKey: in.ImageKey}
		if err := tx.QueryRow(ctx, `
			INSERT INTO seller_support_messages (ticket_id, author_role, author_id, body, image_key)
			VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
			in.TicketID, in.AuthorRole, in.UserID, in.Body, in.ImageKey).
			Scan(&msg.ID, &msg.CreatedAt); err != nil {
			return err
		}
		newStatus = status
		if status == StatusOpen && in.AuthorRole == "admin" {
			newStatus = StatusInProgress
		}
		_, err = tx.Exec(ctx,
			`UPDATE seller_support_tickets SET status=$2, updated_at=now() WHERE id=$1`, in.TicketID, newStatus)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	s.hub.Publish(Event{Scope: scopeSellerSupport, Type: "message", TicketID: in.TicketID, Status: newStatus, Message: &msg},
		Audience{SellerID: ticketSellerID})
	return msg, nil
}

// ResolveSellerTicket marks a seller ticket solved (admin only).
func (s *Service) ResolveSellerTicket(ctx context.Context, ticketID, adminUserID, resolution string) error {
	resolution = strings.TrimSpace(resolution)
	if resolution == "" {
		return httpx.BadRequest("validation_failed", "Add a short resolution note before marking solved")
	}
	if _, err := uuid.Parse(ticketID); err != nil {
		return httpx.NotFound("ticket_not_found", "Support ticket not found")
	}
	var sellerID string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx,
			`SELECT seller_id::text, status FROM seller_support_tickets WHERE id=$1 FOR UPDATE`, ticketID).
			Scan(&sellerID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("ticket_not_found", "Support ticket not found")
		}
		if err != nil {
			return err
		}
		if status == StatusSolved {
			return httpx.Conflict("ticket_solved", "This ticket is already solved")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE seller_support_tickets
			SET status='solved', resolution=$2, resolved_by=$3, resolved_at=now(), updated_at=now()
			WHERE id=$1`, ticketID, resolution, adminUserID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO seller_support_messages (ticket_id, author_role, author_id, body)
			VALUES ($1,'admin',$2,$3)`, ticketID, adminUserID, "Marked solved: "+resolution)
		return err
	})
	if err == nil {
		s.publishSellerChanged(ticketID, sellerID, StatusSolved)
	}
	return err
}

func (s *Service) publishSellerChanged(ticketID, sellerID, status string) {
	s.hub.Publish(Event{Scope: scopeSellerSupport, Type: "ticket_changed", TicketID: ticketID, Status: status},
		Audience{SellerID: sellerID})
}

// ListSellerTicketsForSeller returns one seller's requests (newest first).
func (s *Service) ListSellerTicketsForSeller(ctx context.Context, sellerID string) ([]SellerTicket, error) {
	return s.listSellerTickets(ctx, `WHERE t.seller_id = $1`, sellerID)
}

// ListSellerTicketsForAdmin returns every seller request, optionally by status.
func (s *Service) ListSellerTicketsForAdmin(ctx context.Context, status string) ([]SellerTicket, error) {
	if status == "" {
		return s.listSellerTickets(ctx, `WHERE TRUE`)
	}
	return s.listSellerTickets(ctx, `WHERE t.status = $1`, status)
}

func (s *Service) listSellerTickets(ctx context.Context, where string, args ...any) ([]SellerTicket, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT t.id, t.ticket_number, t.seller_id, coalesce(se.store_name,''),
		       t.category, t.subject, t.description, t.status, t.resolution, t.resolved_at,
		       t.created_at, t.updated_at
		FROM seller_support_tickets t
		LEFT JOIN sellers se ON se.id = t.seller_id
		`+where+` ORDER BY t.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SellerTicket{}
	ids := []string{}
	index := map[string]int{}
	for rows.Next() {
		var t SellerTicket
		if err := rows.Scan(&t.ID, &t.TicketNumber, &t.SellerID, &t.SellerName,
			&t.Category, &t.Subject, &t.Description, &t.Status, &t.Resolution, &t.ResolvedAt,
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

	prows, err := s.repo.pool.Query(ctx,
		`SELECT ticket_id, file_key FROM seller_support_photos WHERE ticket_id = ANY($1) ORDER BY sort_order`, ids)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var tid string
		var p Photo
		if err := prows.Scan(&tid, &p.FileKey); err != nil {
			return nil, err
		}
		if i, ok := index[tid]; ok {
			out[i].Photos = append(out[i].Photos, p)
		}
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}

	mrows, err := s.repo.pool.Query(ctx,
		`SELECT ticket_id, id, author_role, body, image_key, created_at
		 FROM seller_support_messages WHERE ticket_id = ANY($1) ORDER BY created_at`, ids)
	if err != nil {
		return nil, err
	}
	defer mrows.Close()
	for mrows.Next() {
		var tid string
		var m Message
		if err := mrows.Scan(&tid, &m.ID, &m.AuthorRole, &m.Body, &m.ImageKey, &m.CreatedAt); err != nil {
			return nil, err
		}
		if i, ok := index[tid]; ok {
			out[i].Messages = append(out[i].Messages, m)
		}
	}
	return out, mrows.Err()
}

func defaultSellerSubject(category string) string {
	switch category {
	case "payments":
		return "Payment / settlement issue"
	case "orders":
		return "Order / shipping issue"
	case "products":
		return "Product / listing issue"
	case "account":
		return "Account / KYC issue"
	case "technical":
		return "Technical issue"
	default:
		return "Help request"
	}
}

func sellerTicketNumber() string {
	return fmt.Sprintf("SLR-%d", time.Now().UnixNano()%1_000_000_000)
}
