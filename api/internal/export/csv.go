package export

import (
	"encoding/csv"
	"io"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
)

func WriteCSV(w io.Writer, est *estimates.Estimate) error {
	c := csv.NewWriter(w)
	defer c.Flush()
	if err := c.Write([]string{"service", "region", "sku", "quantity", "unit_price", "monthly_cost", "currency", "summary"}); err != nil {
		return err
	}
	for _, it := range est.Items {
		if err := c.Write([]string{
			it.ServiceCode, it.RegionCode, it.SKU,
			it.Quantity.String(), it.UnitPriceSnapshot.String(),
			it.MonthlyCostSnapshot.String(), it.CurrencySnapshot,
			LineSummary(it),
		}); err != nil {
			return err
		}
	}
	return nil
}
