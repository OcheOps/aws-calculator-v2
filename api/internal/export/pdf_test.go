package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/shopspring/decimal"
)

func sample() *estimates.Estimate {
	return &estimates.Estimate{
		Name: "test", Currency: "USD", Total: decimal.NewFromFloat(70.08),
		Items: []estimates.Item{{
			ServiceCode: "AmazonEC2", RegionCode: "us-east-1", SKU: "S",
			Quantity: decimal.NewFromInt(1), UnitPriceSnapshot: decimal.NewFromFloat(0.096),
			MonthlyCostSnapshot: decimal.NewFromFloat(70.08), CurrencySnapshot: "USD",
			Attributes: map[string]string{"instanceType": "m5.large"},
		}},
	}
}

func TestWritePDF_ProducesNonEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePDF(&buf, sample()); err != nil {
		t.Fatalf("WritePDF: %v", err)
	}
	if buf.Len() < 100 || string(buf.Bytes()[:4]) != "%PDF" {
		t.Errorf("output does not look like PDF (%d bytes, prefix=%q)", buf.Len(), buf.Bytes()[:4])
	}
}

func TestWriteCSV_ContainsRow(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, sample()); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	r := csv.NewReader(strings.NewReader(buf.String()))
	rows, _ := r.ReadAll()
	if len(rows) != 2 || rows[1][0] != "AmazonEC2" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestWriteJSON_HasVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, sample()); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var body struct {
		Version  int
		Estimate estimates.Estimate
	}
	if err := json.Unmarshal(buf.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != 1 || body.Estimate.Name != "test" {
		t.Errorf("body = %+v", body)
	}
}

func TestExport_CuratedSummary(t *testing.T) {
	est := &estimates.Estimate{
		Name: "curated", Currency: "USD", Total: decimal.NewFromFloat(140.16),
		Items: []estimates.Item{{
			LineType:    estimates.LineTypeCurated,
			ServiceKey:  "ec2",
			ServiceCode: "AmazonEC2",
			RegionCode:  "us-east-1",
			SKU:         "curated:ec2:v1",
			Quantity:    decimal.NewFromInt(1460),
			UnitPriceSnapshot: decimal.NewFromFloat(0.096),
			MonthlyCostSnapshot: decimal.NewFromFloat(140.16),
			CurrencySnapshot: "USD",
			UserInputSnapshot: map[string]any{
				"count": float64(2), "instanceType": "m5.large",
				"hoursPerMonth": float64(730), "operatingSystem": "Linux",
			},
		}},
	}
	got := LineSummary(est.Items[0])
	if !strings.Contains(got, "EC2") || !strings.Contains(got, "m5.large") || !strings.Contains(got, "Linux") {
		t.Errorf("summary = %q", got)
	}
}
