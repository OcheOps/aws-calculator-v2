package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID              int64
	Email           string
	EmailVerifiedAt *time.Time
}

type Org struct {
	ID   int64
	Name string
	Slug string
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(p *pgxpool.Pool) *Store { return &Store{pool: p} }

func (s *Store) UpsertUser(ctx context.Context, email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email) VALUES ($1)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, email, email_verified_at`, email).Scan(&u.ID, &u.Email, &u.EmailVerifiedAt)
	return &u, err
}

func (s *Store) SaveMagicLink(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, ip, ua string) error {
	var ipVal interface{}
	if ip != "" {
		ipVal = ip
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO magic_link_tokens (user_id, token_hash, expires_at, ip, ua)
		VALUES ($1, $2, $3, $4, $5)`, userID, tokenHash, expiresAt, ipVal, ua)
	return err
}

func (s *Store) ConsumeMagicLink(ctx context.Context, tokenHash string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, `
		UPDATE magic_link_tokens SET used_at = NOW()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
		RETURNING user_id`, tokenHash).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("invalid or expired token")
	}
	_, _ = s.pool.Exec(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, NOW()) WHERE id = $1`, userID)
	return userID, nil
}

func (s *Store) SaveSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	return err
}

func (s *Store) LookupSession(ctx context.Context, tokenHash string) (*User, error) {
	var u User
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.email_verified_at, s.expires_at FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND u.disabled_at IS NULL`, tokenHash).Scan(&u.ID, &u.Email, &u.EmailVerifiedAt, &expiresAt)
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("session expired")
	}
	_, _ = s.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = NOW() WHERE token_hash = $1`, tokenHash)
	return &u, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

var slugSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

func slugify(local string) string {
	local = strings.ToLower(local)
	local = slugSanitizer.ReplaceAllString(local, "-")
	return strings.Trim(local, "-")
}

func (s *Store) EnsureDefaultOrg(ctx context.Context, u *User) (*Org, error) {
	var existing Org
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.slug FROM memberships m
		JOIN organizations o ON o.id = m.org_id
		WHERE m.user_id = $1 ORDER BY m.created_at ASC LIMIT 1`, u.ID).Scan(&existing.ID, &existing.Name, &existing.Slug)
	if err == nil {
		return &existing, nil
	}
	local := u.Email
	if i := strings.Index(u.Email, "@"); i > 0 {
		local = u.Email[:i]
	}
	base := slugify(local)
	if base == "" {
		base = "user"
	}
	name := fmt.Sprintf("%s's workspace", local)
	slug := base
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for i := 0; i < 10; i++ {
		var org Org
		err := tx.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id, name, slug`, name, slug).Scan(&org.ID, &org.Name, &org.Slug)
		if err == nil {
			if _, err := tx.Exec(ctx, `INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, 'owner')`, org.ID, u.ID); err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return &org, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i+1)
	}
	return nil, fmt.Errorf("could not generate unique slug")
}

func (s *Store) GetOrgsForUser(ctx context.Context, userID int64) ([]Org, error) {
	rows, err := s.pool.Query(ctx, `SELECT o.id, o.name, o.slug FROM memberships m JOIN organizations o ON o.id = m.org_id WHERE m.user_id = $1 ORDER BY o.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Org
	for rows.Next() {
		var o Org
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
