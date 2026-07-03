package estimates

import (
	"context"

	"github.com/OcheOps/aws-calculator/api/internal/pricing"
	"github.com/shopspring/decimal"
)

type PriceDelta struct {
	ItemID       int64           `json:"item_id"`
	OldUnitPrice decimal.Decimal `json:"old_unit_price"`
	NewUnitPrice decimal.Decimal `json:"new_unit_price"`
	OldMonthly   decimal.Decimal `json:"old_monthly"`
	NewMonthly   decimal.Decimal `json:"new_monthly"`
	Currency     string          `json:"currency"`
}

type Repricer struct {
	store  *Store
	lookup *pricing.Lookup
}

func NewRepricer(s *Store, l *pricing.Lookup) *Repricer { return &Repricer{store: s, lookup: l} }

func (r *Repricer) Preview(ctx context.Context, estimateID int64) ([]PriceDelta, error) {
	est, err := r.store.Get(ctx, estimateID)
	if err != nil {
		return nil, err
	}
	deltas := make([]PriceDelta, 0, len(est.Items))
	for _, it := range est.Items {
		res, err := r.lookup.Price(ctx, pricing.LookupInput{
			ServiceCode: it.ServiceCode,
			Region:      it.RegionCode,
			Attributes:  it.Attributes,
			Currency:    it.CurrencySnapshot,
			TermType:    "OnDemand",
		})
		if err != nil {
			continue
		}
		newMonthly := MonthlyCost(res.Unit, res.PricePerUnit, it.Quantity)
		deltas = append(deltas, PriceDelta{
			ItemID:       it.ID,
			OldUnitPrice: it.UnitPriceSnapshot,
			NewUnitPrice: res.PricePerUnit,
			OldMonthly:   it.MonthlyCostSnapshot,
			NewMonthly:   newMonthly,
			Currency:     res.Currency,
		})
	}
	return deltas, nil
}

func (r *Repricer) Apply(ctx context.Context, estimateID int64, deltas []PriceDelta) error {
	for _, d := range deltas {
		if _, err := r.store.pool.Exec(ctx, `
			UPDATE estimate_items SET
				unit_price_snapshot = $1,
				monthly_cost_snapshot = $2,
				currency_snapshot = $3
			WHERE id = $4 AND estimate_id = $5`,
			d.NewUnitPrice.String(), d.NewMonthly.String(), d.Currency, d.ItemID, estimateID); err != nil {
			return err
		}
	}
	return nil
}
