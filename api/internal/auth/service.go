package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/mail"
)

// EstimatesClaimer is called from Verify to claim any anonymous estimates the
// client passes as claim_tokens into the user's default org. Wired in Plan 05.
type EstimatesClaimer interface {
	ClaimByEditTokens(ctx context.Context, orgID, userID int64, rawTokens []string) (int, error)
}

type Service struct {
	store      *Store
	mailer     mail.Mailer
	byIP       *Limiter
	byEmail    *Limiter
	baseURL    string
	linkTTL    time.Duration
	sessionTTL time.Duration
	claimer    EstimatesClaimer
}

func NewService(store *Store, m mail.Mailer, byIP, byEmail *Limiter, baseURL string) *Service {
	return &Service{
		store:      store,
		mailer:     m,
		byIP:       byIP,
		byEmail:    byEmail,
		baseURL:    baseURL,
		linkTTL:    15 * time.Minute,
		sessionTTL: 30 * 24 * time.Hour,
	}
}

func (s *Service) WithClaimer(c EstimatesClaimer) *Service {
	s.claimer = c
	return s
}

func (s *Service) RequestLink(ctx context.Context, email, ip, ua string) error {
	if !s.byIP.Allow(ip) || !s.byEmail.Allow(email) {
		return fmt.Errorf("rate limited")
	}
	user, err := s.store.UpsertUser(ctx, email)
	if err != nil {
		return err
	}
	raw, err := GenerateToken()
	if err != nil {
		return err
	}
	if err := s.store.SaveMagicLink(ctx, user.ID, HashToken(raw), time.Now().Add(s.linkTTL), ip, ua); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/auth/verify?token=%s", s.baseURL, raw)
	body := fmt.Sprintf(`<p>Click to sign in:</p><p><a href="%s">%s</a></p><p>This link expires in 15 minutes.</p>`, link, link)
	return s.mailer.Send(user.Email, "Your AWS Calculator sign-in link", body)
}

// Verify consumes a magic-link token, issues a session, ensures a default org,
// and (if a claimer is wired) claims any anonymous estimates matching
// claim_tokens into the user's default org. Returns (user, rawSessionToken,
// expiresAt, error).
func (s *Service) Verify(ctx context.Context, rawToken string, claimTokens []string) (*User, string, time.Time, error) {
	uid, err := s.store.ConsumeMagicLink(ctx, HashToken(rawToken))
	if err != nil {
		return nil, "", time.Time{}, err
	}
	user, err := s.getUser(ctx, uid)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	org, err := s.store.EnsureDefaultOrg(ctx, user)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if s.claimer != nil && len(claimTokens) > 0 {
		_, _ = s.claimer.ClaimByEditTokens(ctx, org.ID, uid, claimTokens)
	}
	sessRaw, err := GenerateToken()
	if err != nil {
		return nil, "", time.Time{}, err
	}
	exp := time.Now().Add(s.sessionTTL)
	if err := s.store.SaveSession(ctx, uid, HashToken(sessRaw), exp); err != nil {
		return nil, "", time.Time{}, err
	}
	return user, sessRaw, exp, nil
}

func (s *Service) Logout(ctx context.Context, sessionRaw string) error {
	return s.store.DeleteSession(ctx, HashToken(sessionRaw))
}

func (s *Service) Session(ctx context.Context, sessionRaw string) (*User, error) {
	return s.store.LookupSession(ctx, HashToken(sessionRaw))
}

func (s *Service) getUser(ctx context.Context, id int64) (*User, error) {
	var u User
	err := s.store.pool.QueryRow(ctx, `SELECT id, email, email_verified_at FROM users WHERE id = $1`, id).Scan(&u.ID, &u.Email, &u.EmailVerifiedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
