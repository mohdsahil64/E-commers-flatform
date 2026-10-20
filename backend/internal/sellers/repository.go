package sellers

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates a missing seller record.
var ErrNotFound = errors.New("not found")

// Repository provides seller persistence.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a seller Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const sellerCols = `id, user_id, store_name, store_slug, store_description, status,
	coalesce(legal_business_name,''), coalesce(business_type,''), coalesce(owner_name,''),
	coalesce(contact_email::text,''), coalesce(contact_phone,''), coalesce(gst_number,''),
	coalesce(pan_number,''), commission_percent, rating_avg, rating_count,
	coalesce(rejection_reason,''), coalesce(suspended_reason,''), resubmit_count,
	onboarding_completed, contract_signed, contract_commission,
	notify_order_emails, notify_return_emails, notify_settlement_emails,
	notify_low_stock, notify_marketing, cod_enabled, created_at`

func scanSeller(row pgx.Row) (*Seller, error) {
	var s Seller
	err := row.Scan(&s.ID, &s.UserID, &s.StoreName, &s.StoreSlug, &s.StoreDescription,
		&s.Status, &s.LegalBusinessName, &s.BusinessType, &s.OwnerName, &s.ContactEmail,
		&s.ContactPhone, &s.GSTNumber, &s.PANNumber, &s.CommissionPercent,
		&s.RatingAvg, &s.RatingCount, &s.RejectionReason, &s.SuspendedReason,
		&s.ResubmitCount, &s.OnboardingDone, &s.ContractSigned, &s.ContractCommission,
		&s.NotifyOrderEmails, &s.NotifyReturnEmails, &s.NotifySettlementEmails,
		&s.NotifyLowStock, &s.NotifyMarketing, &s.CODEnabled, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByUserID returns the seller owned by a user.
func (r *Repository) FindByUserID(ctx context.Context, userID string) (*Seller, error) {
	return scanSeller(r.pool.QueryRow(ctx,
		`SELECT `+sellerCols+` FROM sellers WHERE user_id = $1`, userID))
}

// FindByID returns a seller by id.
func (r *Repository) FindByID(ctx context.Context, id string) (*Seller, error) {
	return scanSeller(r.pool.QueryRow(ctx,
		`SELECT `+sellerCols+` FROM sellers WHERE id = $1`, id))
}

// FindBySlug returns an approved seller by store slug (storefront).
func (r *Repository) FindBySlug(ctx context.Context, slug string) (*Seller, error) {
	return scanSeller(r.pool.QueryRow(ctx,
		`SELECT `+sellerCols+` FROM sellers WHERE store_slug = $1 AND status = 'approved'`, slug))
}

// UpdateProfile updates the fields a seller may edit about their own store:
// display name, description and notification preferences.
func (r *Repository) UpdateProfile(ctx context.Context, sellerID string, p ProfileUpdate) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE sellers SET
			store_name = COALESCE(NULLIF($2,''), store_name),
			store_description = $3,
			notify_order_emails = $4,
			notify_return_emails = $5,
			notify_settlement_emails = $6,
			notify_low_stock = $7,
			notify_marketing = $8,
			cod_enabled = $9
		WHERE id = $1`,
		sellerID, p.StoreName, p.StoreDescription,
		p.NotifyOrderEmails, p.NotifyReturnEmails, p.NotifySettlementEmails,
		p.NotifyLowStock, p.NotifyMarketing, p.CODEnabled)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PasswordHashForUser returns the current bcrypt hash for a user (for verifying
// the current password before a change).
func (r *Repository) PasswordHashForUser(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

// SetPassword updates a user's password hash and revokes their other sessions.
func (r *Repository) SetPassword(ctx context.Context, userID, newHash string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE users SET password_hash = $2 WHERE id = $1`, userID, newHash); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		return err
	})
}

// UpdateBusinessInfo updates business/owner/contact/tax fields during onboarding.
func (r *Repository) UpdateBusinessInfo(ctx context.Context, sellerID string, s Seller) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sellers SET
			legal_business_name = $2, business_type = $3, owner_name = $4,
			contact_email = NULLIF($5,'')::citext, contact_phone = NULLIF($6,''),
			gst_number = NULLIF($7,''), pan_number = NULLIF($8,''),
			store_description = $9
		WHERE id = $1`,
		sellerID, s.LegalBusinessName, s.BusinessType, s.OwnerName,
		s.ContactEmail, s.ContactPhone, s.GSTNumber, s.PANNumber, s.StoreDescription)
	return err
}

// GetOnboarding returns the onboarding record for a seller.
func (r *Repository) GetOnboarding(ctx context.Context, sellerID string) (*Onboarding, error) {
	var o Onboarding
	err := r.pool.QueryRow(ctx, `
		SELECT seller_id, step, business_completed, contact_completed, pickup_completed,
		       bank_completed, documents_completed, contract_completed, coalesce(review_notes,'')
		FROM seller_onboarding WHERE seller_id = $1`, sellerID).
		Scan(&o.SellerID, &o.Step, &o.BusinessCompleted, &o.ContactCompleted,
			&o.PickupCompleted, &o.BankCompleted, &o.DocumentsCompleted,
			&o.ContractCompleted, &o.ReviewNotes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &o, err
}

// MarkOnboardingStep sets a completion flag and current step.
func (r *Repository) MarkOnboardingStep(ctx context.Context, sellerID, step string) error {
	col := map[string]string{
		"business":  "business_completed",
		"owner":     "business_completed",
		"contact":   "contact_completed",
		"pickup":    "pickup_completed",
		"bank":      "bank_completed",
		"documents": "documents_completed",
		"contract":  "contract_completed",
	}[step]
	if col == "" {
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE seller_onboarding SET `+col+` = TRUE, step = $2 WHERE seller_id = $1`,
		sellerID, step)
	return err
}

// SubmitOnboarding marks the seller under_review and records submission time.
// It accepts submissions from a 'pending' (first submit) or 'rejected' (a
// resubmit) seller. When resubmitting after a rejection, the resubmit_count is
// incremented. Returns the seller's new resubmit_count.
func (r *Repository) SubmitOnboarding(ctx context.Context, sellerID string) (int, error) {
	var resubmitCount int
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE seller_onboarding SET step = 'submitted', submitted_at = now() WHERE seller_id = $1`,
			sellerID); err != nil {
			return err
		}
		ct, err := tx.Exec(ctx, `
			UPDATE sellers SET
				status = 'under_review',
				onboarding_completed = TRUE,
				resubmit_count = CASE WHEN status = 'rejected' THEN resubmit_count + 1 ELSE resubmit_count END,
				rejection_reason = NULL
			WHERE id = $1 AND status IN ('pending', 'rejected')`,
			sellerID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		return tx.QueryRow(ctx, `SELECT resubmit_count FROM sellers WHERE id = $1`, sellerID).Scan(&resubmitCount)
	})
	return resubmitCount, err
}

// SetStatus transitions a seller's status (admin action). Reasons are recorded
// for reject/suspend/block; approving clears the rejection reason and (when
// coming back to good standing) the suspended reason too.
func (r *Repository) SetStatus(ctx context.Context, sellerID, status, reason, adminID string) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE sellers SET
			status = $2,
			rejection_reason = CASE WHEN $2 = 'rejected' THEN $3
			                        WHEN $2 = 'approved' THEN NULL
			                        ELSE rejection_reason END,
			suspended_reason = CASE WHEN $2 IN ('suspended','blocked') THEN $3
			                        WHEN $2 = 'approved' THEN NULL
			                        ELSE suspended_reason END,
			approved_at = CASE WHEN $2 = 'approved' THEN now() ELSE approved_at END,
			approved_by = CASE WHEN $2 = 'approved' THEN $4::uuid ELSE approved_by END
		WHERE id = $1`, sellerID, status, reason, adminID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SellerStatus returns just the lifecycle status + resubmit_count for a seller.
func (r *Repository) SellerStatus(ctx context.Context, sellerID string) (string, int, error) {
	var status string
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT status, resubmit_count FROM sellers WHERE id = $1`, sellerID).Scan(&status, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrNotFound
	}
	return status, count, err
}

// SaveBankAccount upserts the seller's primary payout bank account.
func (r *Repository) SaveBankAccount(ctx context.Context, sellerID string, b BankAccount) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM seller_bank_accounts WHERE seller_id = $1`, sellerID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO seller_bank_accounts (seller_id, account_holder, account_number, ifsc, bank_name, branch, is_primary)
			VALUES ($1,$2,$3,$4,$5,$6,TRUE)`,
			sellerID, b.AccountHolder, b.AccountNumber, b.IFSC, b.BankName, b.Branch)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`UPDATE seller_onboarding SET bank_completed = TRUE WHERE seller_id = $1`, sellerID)
		return err
	})
}

// GetBankAccount returns the seller's primary bank account (nil if none).
func (r *Repository) GetBankAccount(ctx context.Context, sellerID string) (*BankAccount, error) {
	var b BankAccount
	err := r.pool.QueryRow(ctx, `
		SELECT account_holder, account_number, ifsc, coalesce(bank_name,''), coalesce(branch,'')
		FROM seller_bank_accounts WHERE seller_id = $1 ORDER BY is_primary DESC, created_at DESC LIMIT 1`,
		sellerID).Scan(&b.AccountHolder, &b.AccountNumber, &b.IFSC, &b.BankName, &b.Branch)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// SavePickupAddress upserts the seller's default pickup / store location.
func (r *Repository) SavePickupAddress(ctx context.Context, sellerID string, a PickupAddress) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM seller_pickup_addresses WHERE seller_id = $1`, sellerID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO seller_pickup_addresses
				(seller_id, label, contact_name, contact_phone, line1, line2, city, state, pincode, country, is_default)
			VALUES ($1,COALESCE(NULLIF($2,''),'Primary'),$3,$4,$5,$6,$7,$8,$9,COALESCE(NULLIF($10,''),'India'),TRUE)`,
			sellerID, a.Label, a.ContactName, a.ContactPhone, a.Line1, a.Line2, a.City, a.State, a.Pincode, a.Country)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`UPDATE seller_onboarding SET pickup_completed = TRUE WHERE seller_id = $1`, sellerID)
		return err
	})
}

// GetPickupAddress returns the seller's default pickup address (nil if none).
func (r *Repository) GetPickupAddress(ctx context.Context, sellerID string) (*PickupAddress, error) {
	var a PickupAddress
	err := r.pool.QueryRow(ctx, `
		SELECT coalesce(label,''), contact_name, contact_phone, line1, coalesce(line2,''),
		       city, state, pincode, coalesce(country,'India')
		FROM seller_pickup_addresses WHERE seller_id = $1 ORDER BY is_default DESC, created_at DESC LIMIT 1`,
		sellerID).Scan(&a.Label, &a.ContactName, &a.ContactPhone, &a.Line1, &a.Line2,
		&a.City, &a.State, &a.Pincode, &a.Country)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SignContract records the seller's acceptance of the commission agreement.
func (r *Repository) SignContract(ctx context.Context, sellerID, signedName, version string, commission float64) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE sellers SET
				contract_signed = TRUE,
				contract_signed_at = now(),
				contract_signed_name = $2,
				contract_version = $3,
				contract_commission = $4,
				commission_percent = COALESCE(commission_percent, $4),
				agreement_accepted = TRUE,
				agreement_accepted_at = now()
			WHERE id = $1`, sellerID, signedName, version, commission); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE seller_onboarding SET contract_completed = TRUE WHERE seller_id = $1`, sellerID)
		return err
	})
}

// UpdateOwnerInfo saves owner / business / tax details during onboarding.
func (r *Repository) UpdateOwnerInfo(ctx context.Context, sellerID string, s Seller) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE sellers SET
				legal_business_name = $2, business_type = $3, owner_name = $4,
				contact_email = NULLIF($5,'')::citext, contact_phone = NULLIF($6,''),
				gst_number = NULLIF($7,''), pan_number = NULLIF($8,''),
				store_description = COALESCE(NULLIF($9,''), store_description)
			WHERE id = $1`,
			sellerID, s.LegalBusinessName, s.BusinessType, s.OwnerName,
			s.ContactEmail, s.ContactPhone, s.GSTNumber, s.PANNumber, s.StoreDescription); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE seller_onboarding SET business_completed = TRUE WHERE seller_id = $1`, sellerID)
		return err
	})
}

// List returns sellers filtered by status with pagination (admin).
func (r *Repository) List(ctx context.Context, status string, limit, offset int) ([]Seller, int64, error) {
	args := []any{}
	where := ""
	if status != "" {
		args = append(args, status)
		where = "WHERE status = $1"
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM sellers `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	q := `SELECT ` + sellerCols + ` FROM sellers ` + where +
		` ORDER BY created_at DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Seller
	for rows.Next() {
		s, err := scanSeller(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
