package fx

import (
	"time"

	"github.com/shopspring/decimal"
)

type Rate struct {
	Base      string          `json:"base"`
	Quote     string          `json:"quote"`
	Rate      decimal.Decimal `json:"rate"`
	FetchedAt time.Time       `json:"fetched_at"`
}
