package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/identity"
	"beta-go/api/internal/platform/config"
)

// UserDirectory is the identity storage sign-in is allowed to touch.
type UserDirectory interface {
	UpsertGoogleUser(ctx context.Context, profile identity.GoogleProfile, extraRoles []string) (identity.User, error)
	Find(ctx context.Context, id uuid.UUID) (identity.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, name, phone, area string) (identity.User, error)
}

// Sessions is the refresh-session storage sign-in is allowed to touch.
type Sessions interface {
	Create(ctx context.Context, userID uuid.UUID, refreshHash string, expiresAt time.Time, userAgent, ip string) (uuid.UUID, error)
	FindLive(ctx context.Context, refreshHash string) (Session, error)
	Rotate(ctx context.Context, currentHash, nextHash string, expiresAt time.Time) (Session, error)
	Revoke(ctx context.Context, refreshHash string) error
	Active(ctx context.Context, id uuid.UUID) error
}

// Service turns a verified Google account into a Beta-Go session.
type Service struct {
	cfg      config.Config
	users    UserDirectory
	sessions Sessions
	google   Verifier
	now      func() time.Time
}

var (
	_ UserDirectory = (*identity.Store)(nil)
	_ Sessions      = (*SessionStore)(nil)
)

func NewService(cfg config.Config, users UserDirectory, sessions Sessions, google Verifier) *Service {
	return &Service{
		cfg:      cfg,
		users:    users,
		sessions: sessions,
		google:   google,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// SignedIn is the result of a successful sign-in or refresh.
type SignedIn struct {
	User         identity.User
	AccessToken  string
	RefreshToken string
}

// SignInWithGoogle verifies the Google token, opens a customer account, and starts a session.
// Staff roles are never chosen by the caller. super_admin comes only from BOOTSTRAP_ADMIN_EMAILS.
func (s *Service) SignInWithGoogle(ctx context.Context, idToken, userAgent, ip string) (SignedIn, error) {
	profile, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return SignedIn{}, ErrUnauthorized
	}

	var extra []string
	if s.cfg.IsBootstrapAdmin(profile.Email) {
		extra = append(extra, identity.RoleSuperAdmin)
	}

	user, err := s.users.UpsertGoogleUser(ctx, profile, extra)
	if err != nil {
		return SignedIn{}, err
	}
	if user.Disabled {
		return SignedIn{}, ErrAccountDisabled
	}
	return s.startSession(ctx, user, userAgent, ip)
}

// Refresh rotates the refresh token and issues a new access token.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (SignedIn, error) {
	if refreshToken == "" {
		return SignedIn{}, ErrUnauthorized
	}
	currentHash := hashToken(refreshToken)
	session, err := s.sessions.FindLive(ctx, currentHash)
	if err != nil {
		return SignedIn{}, err
	}
	user, err := s.users.Find(ctx, session.UserID)
	if err != nil {
		return SignedIn{}, ErrUnauthorized
	}
	if user.Disabled {
		_ = s.sessions.Revoke(ctx, currentHash)
		return SignedIn{}, ErrAccountDisabled
	}
	nextRaw, nextHash, err := newRefreshToken()
	if err != nil {
		return SignedIn{}, err
	}
	session, err = s.sessions.Rotate(ctx, currentHash, nextHash, s.now().Add(s.cfg.RefreshTTL))
	if err != nil {
		return SignedIn{}, err
	}
	access, err := issueAccess(s.cfg.SessionSecret, user, session.ID, s.cfg.AccessTTL)
	if err != nil {
		return SignedIn{}, err
	}
	return SignedIn{User: user, AccessToken: access, RefreshToken: nextRaw}, nil
}

// Logout revokes the refresh session. A missing cookie still counts as signed out.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.sessions.Revoke(ctx, hashToken(refreshToken))
}

// UpdateProfile stores name, phone, and area after Google has opened the account.
func (s *Service) UpdateProfile(ctx context.Context, principal Principal, name, phone, area string) (identity.User, error) {
	user, err := s.users.UpdateProfile(ctx, principal.UserID, name, phone, area)
	if err != nil {
		return identity.User{}, err
	}
	if user.Disabled {
		return identity.User{}, ErrAccountDisabled
	}
	return user, nil
}

// CurrentUser loads the person named by a valid access token.
func (s *Service) CurrentUser(ctx context.Context, principal Principal) (identity.User, error) {
	user, err := s.users.Find(ctx, principal.UserID)
	if err != nil {
		return identity.User{}, ErrUnauthorized
	}
	if user.Disabled {
		return identity.User{}, ErrAccountDisabled
	}
	return user, nil
}

// StartSession opens a new session for a person who is already in the database.
// Accepting a vendor invitation uses this so the new role is on the next cookie.
func (s *Service) StartSession(ctx context.Context, userID uuid.UUID, userAgent, ip string) (SignedIn, error) {
	user, err := s.users.Find(ctx, userID)
	if err != nil {
		return SignedIn{}, ErrUnauthorized
	}
	if user.Disabled {
		return SignedIn{}, ErrAccountDisabled
	}
	return s.startSession(ctx, user, userAgent, ip)
}

func (s *Service) startSession(ctx context.Context, user identity.User, userAgent, ip string) (SignedIn, error) {
	raw, hash, err := newRefreshToken()
	if err != nil {
		return SignedIn{}, err
	}
	sessionID, err := s.sessions.Create(ctx, user.ID, hash, s.now().Add(s.cfg.RefreshTTL), userAgent, ip)
	if err != nil {
		return SignedIn{}, err
	}
	access, err := issueAccess(s.cfg.SessionSecret, user, sessionID, s.cfg.AccessTTL)
	if err != nil {
		return SignedIn{}, err
	}
	slog.Info("session started", "user_id", user.ID.String())
	return SignedIn{User: user, AccessToken: access, RefreshToken: raw}, nil
}
