package teams

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/entitlements"
)

var ErrInvalidInvite = errors.New("invalid team invitation")

type Invitation struct {
	ID             string
	OrganizationID string
	Email          string
	Role           entitlements.Role
	Token          string
	TokenHash      []byte
	ExpiresAt      time.Time
	AcceptedAt     time.Time
	CreatedAt      time.Time
}

func NewInvitation(email string, role entitlements.Role) (Invitation, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return Invitation{}, err
	}
	if role == "" {
		role = entitlements.RoleViewer
	}
	token, hash, err := newToken()
	if err != nil {
		return Invitation{}, err
	}
	return Invitation{Email: normalized, Role: role, Token: token, TokenHash: hash}, nil
}

type DomainTransfer struct {
	DomainID          string
	FromOrganization  string
	ToOrganization    string
	RequestedByUserID string
}

type TransferStatus string

const (
	TransferPending  TransferStatus = "pending"
	TransferAccepted TransferStatus = "accepted"
	TransferRejected TransferStatus = "rejected"
)

type TransferRequest struct {
	DomainTransfer
	ID          string
	Status      TransferStatus
	RequestedAt time.Time
	DecidedAt   time.Time
	DecidedBy   string
}

func ValidateTransfer(transfer DomainTransfer, requesterRole entitlements.Role) error {
	if transfer.DomainID == "" || transfer.FromOrganization == "" || transfer.ToOrganization == "" || transfer.RequestedByUserID == "" {
		return fmt.Errorf("domain transfer requires domain, source organization, target organization, and requester")
	}
	if transfer.FromOrganization == transfer.ToOrganization {
		return fmt.Errorf("domain transfer target must differ from source")
	}
	if !entitlements.HasPermission(requesterRole, entitlements.PermissionManageDomain) {
		return fmt.Errorf("requester cannot transfer domains")
	}
	return nil
}

func NewTransferRequest(transfer DomainTransfer, requesterRole entitlements.Role, requestedAt time.Time) (TransferRequest, error) {
	if err := ValidateTransfer(transfer, requesterRole); err != nil {
		return TransferRequest{}, err
	}
	return TransferRequest{DomainTransfer: transfer, Status: TransferPending, RequestedAt: requestedAt.UTC()}, nil
}

func DecideTransfer(request TransferRequest, decidedBy string, accept bool, decidedAt time.Time) (TransferRequest, error) {
	if request.Status != TransferPending {
		return TransferRequest{}, fmt.Errorf("domain transfer is already decided")
	}
	decidedBy = strings.TrimSpace(decidedBy)
	if decidedBy == "" {
		return TransferRequest{}, fmt.Errorf("decider is required")
	}
	request.DecidedBy = decidedBy
	request.DecidedAt = decidedAt.UTC()
	if accept {
		request.Status = TransferAccepted
	} else {
		request.Status = TransferRejected
	}
	return request, nil
}

func normalizeEmail(input string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(input))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidInvite
	}
	return email, nil
}

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}
