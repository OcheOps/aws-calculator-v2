package pricing

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCache_GetSet(t *testing.T) {
	c, err := NewCache(1<<12, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	key := "svc|us-east-1|OnDemand|instanceType=m5.large"
	val := &LookupResult{SKU: "S1", PricePerUnit: decimal.NewFromFloat(0.1)}
	c.Set(key, val)
	time.Sleep(10 * time.Millisecond)
	got, ok := c.Get(key)
	if !ok || got.SKU != "S1" {
		t.Errorf("got=%v ok=%v", got, ok)
	}
}
