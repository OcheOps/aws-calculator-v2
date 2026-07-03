package providers

import (
	"context"
	"testing"
)

type stubProvider struct {
	code       string
	ingestErr  error
	ingestSeen bool
}

func (s *stubProvider) Metadata() Metadata { return Metadata{Code: s.code, Name: "stub"} }
func (s *stubProvider) Ingest(ctx context.Context, deps IngestDeps) error {
	s.ingestSeen = true
	return s.ingestErr
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	p := &stubProvider{code: "aws"}
	r.Register(p)

	got, ok := r.Get("aws")
	if !ok || got.Metadata().Code != "aws" {
		t.Fatalf("Get(aws) = %v, %v", got, ok)
	}
	if _, ok := r.Get("gcp"); ok {
		t.Error("expected gcp to be missing")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubProvider{code: "aws"})
	r.Register(&stubProvider{code: "gcp"})
	all := r.All()
	if len(all) != 2 {
		t.Errorf("len(All()) = %d, want 2", len(all))
	}
}

func TestRegistry_RegisterPanicsOnDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubProvider{code: "aws"})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	r.Register(&stubProvider{code: "aws"})
}
