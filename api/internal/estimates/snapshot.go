package estimates

import (
	"strings"

	"github.com/shopspring/decimal"
)

// MonthlyCost multiplies unit price by quantity, then by the appropriate
// per-month multiplier for the unit.
func MonthlyCost(unit string, unitPrice, quantity decimal.Decimal) decimal.Decimal {
	multiplier := decimal.NewFromInt(1)
	switch strings.ToLower(unit) {
	case "hrs", "hour", "hours":
		multiplier = decimal.NewFromInt(HoursPerMonth)
	}
	return unitPrice.Mul(quantity).Mul(multiplier)
}
