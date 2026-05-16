package teams

import (
	"errors"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/entitlements"
)

func TestNewInvitation(t *testing.T) {
	t.Parallel()

	invite, err := NewInvitation(" Person@Example.COM ", entitlements.RoleAdmin)
	if err != nil {
		t.Fatalf("NewInvitation returned error: %v", err)
	}
	if invite.Email != "person@example.com" || invite.Role != entitlements.RoleAdmin || invite.Token == "" || len(invite.TokenHash) == 0 {
		t.Fatalf("unexpected invitation: %+v", invite)
	}
}

func TestNewInvitationRejectsInvalidEmail(t *testing.T) {
	t.Parallel()

	if _, err := NewInvitation("not an email", ""); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("NewInvitation error = %v, want ErrInvalidInvite", err)
	}
}

func TestValidateTransfer(t *testing.T) {
	t.Parallel()

	transfer := DomainTransfer{
		DomainID:          "domain-1",
		FromOrganization:  "org-1",
		ToOrganization:    "org-2",
		RequestedByUserID: "user-1",
	}
	if err := ValidateTransfer(transfer, entitlements.RoleAdmin); err != nil {
		t.Fatalf("ValidateTransfer returned error: %v", err)
	}
	if err := ValidateTransfer(transfer, entitlements.RoleViewer); err == nil {
		t.Fatalf("ValidateTransfer returned nil error for viewer")
	}
	transfer.ToOrganization = "org-1"
	if err := ValidateTransfer(transfer, entitlements.RoleOwner); err == nil {
		t.Fatalf("ValidateTransfer returned nil error for same target")
	}
}

func TestTransferRequestDecision(t *testing.T) {
	t.Parallel()

	requestedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	request, err := NewTransferRequest(DomainTransfer{
		DomainID:          "domain-1",
		FromOrganization:  "org-1",
		ToOrganization:    "org-2",
		RequestedByUserID: "user-1",
	}, entitlements.RoleAdmin, requestedAt)
	if err != nil {
		t.Fatalf("NewTransferRequest returned error: %v", err)
	}
	if request.Status != TransferPending || !request.RequestedAt.Equal(requestedAt) {
		t.Fatalf("unexpected request: %+v", request)
	}
	decided, err := DecideTransfer(request, "owner-1", true, requestedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("DecideTransfer returned error: %v", err)
	}
	if decided.Status != TransferAccepted || decided.DecidedBy != "owner-1" || decided.DecidedAt.IsZero() {
		t.Fatalf("unexpected decision: %+v", decided)
	}
	if _, err := DecideTransfer(decided, "owner-1", false, requestedAt.Add(2*time.Hour)); err == nil {
		t.Fatalf("DecideTransfer returned nil error for already decided transfer")
	}
}
