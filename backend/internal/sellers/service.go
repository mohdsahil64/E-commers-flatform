package sellers

import (
	"context"
	"strings"

	"github.com/carlitos/backend/internal/httpx"
)

// LifecycleHook is called after a seller status change so other modules
// (audit, notifications) can react without the sellers package depending on them.
type LifecycleHook func(ctx context.Context, sellerID, event, actorID string)

// ContractVersion identifies the commission-agreement text a seller signed.
// Bump this when the agreement wording changes.
const ContractVersion = "2024-01"

// Service holds seller business logic.
type Service struct {
	repo              *Repository
	hook              LifecycleHook
	defaultCommission float64
	bcryptCost        int
}

// NewService builds a seller Service. hook may be nil.
func NewService(repo *Repository, hook LifecycleHook) *Service {
	if hook == nil {
		hook = func(context.Context, string, string, string) {}
	}
	return &Service{repo: repo, hook: hook, defaultCommission: 12.0, bcryptCost: 10}
}

// WithBcryptCost sets the cost used when hashing a changed password.
func (s *Service) WithBcryptCost(cost int) *Service {
	if cost > 0 {
		s.bcryptCost = cost
	}
	return s
}

// WithDefaultCommission sets the marketplace default commission percent shown
// in (and stored with) the seller contract.
func (s *Service) WithDefaultCommission(pct float64) *Service {
	if pct > 0 {
		s.defaultCommission = pct
	}
	return s
}

// DefaultCommission returns the marketplace default commission percent.
func (s *Service) DefaultCommission() float64 { return s.defaultCommission }

// Repo exposes the repository for lookups used by other modules.
func (s *Service) Repo() *Repository { return s.repo }

// GetByUser returns the seller profile for a user.
func (s *Service) GetByUser(ctx context.Context, userID string) (*Seller, error) {
	seller, err := s.repo.FindByUserID(ctx, userID)
	if err == ErrNotFound {
		return nil, httpx.NotFound("seller_not_found", "Seller profile not found")
	}
	return seller, err
}

// GetOnboarding returns onboarding progress.
func (s *Service) GetOnboarding(ctx context.Context, sellerID string) (*Onboarding, error) {
	o, err := s.repo.GetOnboarding(ctx, sellerID)
	if err == ErrNotFound {
		return nil, httpx.NotFound("onboarding_not_found", "Onboarding not started")
	}
	return o, err
}

// SaveBusinessInfo updates business info and marks the business step complete.
func (s *Service) SaveBusinessInfo(ctx context.Context, sellerID string, in Seller) error {
	if in.LegalBusinessName == "" || in.OwnerName == "" {
		return httpx.BadRequest("validation_failed", "Business name and owner name are required")
	}
	if err := s.repo.UpdateBusinessInfo(ctx, sellerID, in); err != nil {
		return err
	}
	return s.repo.MarkOnboardingStep(ctx, sellerID, "business")
}

// CompleteStep marks an onboarding step complete.
func (s *Service) CompleteStep(ctx context.Context, sellerID, step string) error {
	return s.repo.MarkOnboardingStep(ctx, sellerID, step)
}

// GetBundle returns the seller profile + onboarding + saved bank & pickup so the
// UI can prefill and never re-ask data the seller already provided.
func (s *Service) GetBundle(ctx context.Context, sellerID string) (*OnboardingBundle, error) {
	seller, err := s.repo.FindByID(ctx, sellerID)
	if err == ErrNotFound {
		return nil, httpx.NotFound("seller_not_found", "Seller not found")
	} else if err != nil {
		return nil, err
	}
	onb, err := s.repo.GetOnboarding(ctx, sellerID)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	bank, err := s.repo.GetBankAccount(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	pickup, err := s.repo.GetPickupAddress(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return &OnboardingBundle{Seller: seller, Onboarding: onb, Bank: bank, Pickup: pickup}, nil
}

// storeNameEditable reports whether a seller may still change their store name.
// Once the store is submitted for review or approved, the name is locked (it is
// customer-facing and tied to the public store slug).
func storeNameEditable(status string) bool {
	return status == "pending" || status == "rejected"
}

// UpdateProfile lets a seller edit their own store description and notification
// preferences. The store name is only accepted while the store is still
// editable (pending/rejected); for approved/under_review stores it is ignored,
// so an approved store name can never be changed.
func (s *Service) UpdateProfile(ctx context.Context, sellerID string, in ProfileUpdate) (*Seller, error) {
	seller, err := s.repo.FindByID(ctx, sellerID)
	if err == ErrNotFound {
		return nil, httpx.NotFound("seller_not_found", "Seller not found")
	} else if err != nil {
		return nil, err
	}
	if !storeNameEditable(seller.Status) {
		// Lock the name to its current value regardless of what was sent.
		in.StoreName = seller.StoreName
	} else if strings.TrimSpace(in.StoreName) == "" {
		return nil, httpx.BadRequest("validation_failed", "Store name is required")
	} else {
		in.StoreName = strings.TrimSpace(in.StoreName)
	}
	// Resolve the COD flag: honour an explicit value from the client, otherwise
	// keep the seller's current setting unchanged.
	if in.CODEnabledPtr != nil {
		in.CODEnabled = *in.CODEnabledPtr
	} else {
		in.CODEnabled = seller.CODEnabled
	}
	if err := s.repo.UpdateProfile(ctx, sellerID, in); err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("seller_not_found", "Seller not found")
		}
		return nil, err
	}
	return s.repo.FindByID(ctx, sellerID)
}

// ChangePassword verifies the current password and sets a new strong one for
// the seller's user account, revoking other sessions.
func (s *Service) ChangePassword(ctx context.Context, userID, current, next string) error {
	current = strings.TrimSpace(current)
	if current == "" {
		return httpx.BadRequest("validation_failed", "Current password is required")
	}
	if msg := StrongPasswordError(next); msg != "" {
		return httpx.BadRequest("weak_password", msg)
	}
	hash, err := s.repo.PasswordHashForUser(ctx, userID)
	if err == ErrNotFound {
		return httpx.NotFound("user_not_found", "User not found")
	} else if err != nil {
		return err
	}
	if !checkPassword(hash, current) {
		return httpx.Unauthorized("invalid_password", "Your current password is incorrect")
	}
	newHash, err := hashPassword(next, s.bcryptCost)
	if err != nil {
		return httpx.Internal("failed to hash password")
	}
	return s.repo.SetPassword(ctx, userID, newHash)
}

// onboardingEditable reports whether the seller may still change onboarding
// data (owner, bank, pickup, contract). Once the application is submitted
// (under_review) or decided (approved/suspended/blocked), these are LOCKED so
// important details like the pickup address can't be changed afterwards. A
// rejected seller may edit again while resubmitting.
func onboardingEditable(status string) bool {
	return status == "pending" || status == "rejected"
}

// ensureOnboardingEditable returns a 403 if the seller's onboarding data is
// locked (already submitted/approved).
func (s *Service) ensureOnboardingEditable(ctx context.Context, sellerID string) error {
	seller, err := s.repo.FindByID(ctx, sellerID)
	if err == ErrNotFound {
		return httpx.NotFound("seller_not_found", "Seller not found")
	} else if err != nil {
		return err
	}
	if !onboardingEditable(seller.Status) {
		return httpx.Forbidden("onboarding_locked", "Your details are locked after submission. Contact support to change them.")
	}
	return nil
}

// SaveOwnerInfo persists owner/business/tax details for the onboarding.
func (s *Service) SaveOwnerInfo(ctx context.Context, sellerID string, in Seller) error {
	if err := s.ensureOnboardingEditable(ctx, sellerID); err != nil {
		return err
	}
	if in.OwnerName == "" {
		return httpx.BadRequest("validation_failed", "Owner name is required")
	}
	return s.repo.UpdateOwnerInfo(ctx, sellerID, in)
}

// SaveBank validates and persists the seller payout bank account.
func (s *Service) SaveBank(ctx context.Context, sellerID string, b BankAccount) error {
	if err := s.ensureOnboardingEditable(ctx, sellerID); err != nil {
		return err
	}
	if b.AccountHolder == "" || len(b.AccountNumber) < 8 || b.IFSC == "" {
		return httpx.BadRequest("validation_failed", "Account holder, a valid account number and IFSC are required")
	}
	return s.repo.SaveBankAccount(ctx, sellerID, b)
}

// SavePickup validates and persists the seller pickup / store location.
func (s *Service) SavePickup(ctx context.Context, sellerID string, a PickupAddress) error {
	if err := s.ensureOnboardingEditable(ctx, sellerID); err != nil {
		return err
	}
	if a.ContactName == "" || a.ContactPhone == "" || a.Line1 == "" || a.City == "" || a.State == "" || a.Pincode == "" {
		return httpx.BadRequest("validation_failed", "Contact, address, city, state and pincode are required")
	}
	return s.repo.SavePickupAddress(ctx, sellerID, a)
}

// SignContract records the seller's acceptance of the commission agreement at
// the marketplace default rate.
func (s *Service) SignContract(ctx context.Context, sellerID, signedName string) error {
	if err := s.ensureOnboardingEditable(ctx, sellerID); err != nil {
		return err
	}
	if signedName == "" {
		return httpx.BadRequest("validation_failed", "Please type your full legal name to sign")
	}
	return s.repo.SignContract(ctx, sellerID, signedName, ContractVersion, s.defaultCommission)
}

// SubmitApplication submits onboarding for admin review. It enforces that the
// required steps (owner details, bank, pickup, signed contract) are done, and
// applies the resubmit cap: after MaxResubmits rejected attempts the account is
// permanently blocked instead of going back under review.
func (s *Service) SubmitApplication(ctx context.Context, sellerID string) error {
	seller, err := s.repo.FindByID(ctx, sellerID)
	if err == ErrNotFound {
		return httpx.NotFound("seller_not_found", "Seller not found")
	} else if err != nil {
		return err
	}
	if seller.Status == "blocked" {
		return httpx.Forbidden("seller_blocked", "Your account is blocked and cannot be resubmitted")
	}
	if seller.Status != "pending" && seller.Status != "rejected" {
		return httpx.BadRequest("not_submittable", "Your application cannot be submitted in its current state")
	}
	// A rejected seller who has already used all attempts is permanently blocked.
	if seller.Status == "rejected" && seller.ResubmitCount >= MaxResubmits {
		_ = s.repo.SetStatus(ctx, sellerID, "blocked",
			"Application rejected after the maximum number of attempts.", "")
		s.hook(ctx, sellerID, "seller.blocked", "")
		return httpx.Forbidden("resubmit_limit", "You have reached the maximum number of resubmissions. Your account is now blocked.")
	}
	onb, err := s.repo.GetOnboarding(ctx, sellerID)
	if err != nil {
		return httpx.BadRequest("onboarding_incomplete", "Please complete onboarding before submitting")
	}
	if !onb.BusinessCompleted || !onb.BankCompleted || !onb.PickupCompleted || !onb.ContractCompleted {
		return httpx.BadRequest("onboarding_incomplete", "Complete owner details, bank, pickup location and the contract before submitting")
	}
	if _, err := s.repo.SubmitOnboarding(ctx, sellerID); err != nil {
		if err == ErrNotFound {
			return httpx.BadRequest("not_submittable", "Your application cannot be submitted in its current state")
		}
		return err
	}
	s.hook(ctx, sellerID, "seller.submitted", "")
	return nil
}

// ---- admin ----

// List returns sellers for admin management.
func (s *Service) List(ctx context.Context, status string, limit, offset int) ([]Seller, int64, error) {
	return s.repo.List(ctx, status, limit, offset)
}

// Get returns a seller by id (admin).
func (s *Service) Get(ctx context.Context, id string) (*Seller, error) {
	seller, err := s.repo.FindByID(ctx, id)
	if err == ErrNotFound {
		return nil, httpx.NotFound("seller_not_found", "Seller not found")
	}
	return seller, err
}

// transition applies an admin status change and fires the lifecycle hook.
func (s *Service) transition(ctx context.Context, sellerID, status, reason, event, adminID string) error {
	if err := s.repo.SetStatus(ctx, sellerID, status, reason, adminID); err == ErrNotFound {
		return httpx.NotFound("seller_not_found", "Seller not found")
	} else if err != nil {
		return err
	}
	s.hook(ctx, sellerID, event, adminID)
	return nil
}

func (s *Service) Approve(ctx context.Context, sellerID, adminID string) error {
	return s.transition(ctx, sellerID, "approved", "", "seller.approved", adminID)
}
func (s *Service) Reject(ctx context.Context, sellerID, reason, adminID string) error {
	if reason == "" {
		return httpx.BadRequest("validation_failed", "Rejection reason is required")
	}
	return s.transition(ctx, sellerID, "rejected", reason, "seller.rejected", adminID)
}
func (s *Service) Suspend(ctx context.Context, sellerID, reason, adminID string) error {
	return s.transition(ctx, sellerID, "suspended", reason, "seller.suspended", adminID)
}
func (s *Service) Activate(ctx context.Context, sellerID, adminID string) error {
	return s.transition(ctx, sellerID, "approved", "", "seller.activated", adminID)
}
func (s *Service) Block(ctx context.Context, sellerID, reason, adminID string) error {
	return s.transition(ctx, sellerID, "blocked", reason, "seller.blocked", adminID)
}

// Unsuspend lifts a suspension, returning the seller to approved (active).
func (s *Service) Unsuspend(ctx context.Context, sellerID, adminID string) error {
	return s.transition(ctx, sellerID, "approved", "", "seller.unsuspended", adminID)
}

// Unblock lifts a block. The seller returns to approved standing (admin's call).
func (s *Service) Unblock(ctx context.Context, sellerID, adminID string) error {
	return s.transition(ctx, sellerID, "approved", "", "seller.unblocked", adminID)
}
