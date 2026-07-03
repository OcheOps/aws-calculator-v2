package export

import (
	"encoding/csv"
	"io"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
)

func WriteCSV(w io.Writer, est *estimates.Estimate) error {
	c := csv.NewWriter(w)
	defer c.Flush()
	if err := c.Write([]string{"service", "region", "sku", "quantity", "unit_price", "monthly_cost", "currency", "attributes"}); err != nil {
		return err
	}
	for _, it := range est.Items {
		attrs := ""
		for k, v := range it.Attributes {
			if attrs != "" {
				attrs += ","
			}
			attrs += k + "=" + v
		}
		if err := c.Write([]string{
			it.ServiceCode, it.RegionCode, it.SKU,
			it.Quantity.String(), it.UnitPriceSnapshot.String(),
			it.MonthlyCostSnapshot.String(), it.CurrencySnapshot, attrs,
		}); err != nil {
			return err
		}
	}
	return nil
}
