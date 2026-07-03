package providers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Metadata struct {
	Code string
	Name string
}

type IngestDeps struct {
	DB     *pgxpool.Pool
	Logger *slog.Logger
	RunID  int64
}

type Provider interface {
	Metadata() Metadata
	Ingest(ctx context.Context, deps IngestDeps) error
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	code := p.Metadata().Code
	if _, exists := r.providers[code]; exists {
		panic(fmt.Sprintf("provider %q already registered", code))
	}
	r.providers[code] = p
}

func (r *Registry) Get(code string) (Provider, bool) {
	p, ok := r.providers[code]
	return p, ok
}

func (r *Registry) All() []Provider {
	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out
}
