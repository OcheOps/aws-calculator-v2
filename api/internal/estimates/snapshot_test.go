package estimates

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestMonthlyCost_Hours(t *testing.T) {
	got := MonthlyCost("Hrs", decimal.NewFromFloat(0.096), decimal.NewFromInt(1))
	want := decimal.NewFromFloat(0.096).Mul(decimal.NewFromInt(730))
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestMonthlyCost_GBMo(t *testing.T) {
	got := MonthlyCost("GB-Mo", decimal.NewFromFloat(0.023), decimal.NewFromInt(100))
	if !got.Equal(decimal.NewFromFloat(2.3)) {
		t.Errorf("got %s", got)
	}
}
