package fx

import "context"

type Refresher struct {
	fetcher Fetcher
	store   *Store
	quotes  []string
}

func NewRefresher(f Fetcher, s *Store, quotes []string) *Refresher {
	return &Refresher{fetcher: f, store: s, quotes: quotes}
}

func (r *Refresher) Refresh(ctx context.Context) error {
	rates, err := r.fetcher.Fetch(ctx, "USD", r.quotes)
	if err != nil {
		return err
	}
	return r.store.Upsert(ctx, rates)
}
