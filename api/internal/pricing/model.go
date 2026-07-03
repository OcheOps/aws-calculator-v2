package pricing

import "github.com/shopspring/decimal"

type LookupQuery struct {
	ServiceID   int64
	ServiceCode string
	Region      string
	TermType    string
	Attributes  map[string]string
}

type LookupResult struct {
	SKU          string          `json:"sku"`
	Unit         string          `json:"unit"`
	Currency     string          `json:"currency"`
	PricePerUnit decimal.Decimal `json:"price_per_unit"`
	TermType     string          `json:"term_type"`
	MatchQuality string          `json:"match_quality"`
	DroppedAttrs []string        `json:"dropped_attributes,omitempty"`
}
