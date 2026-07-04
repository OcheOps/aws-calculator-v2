package estimates

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/shopspring/decimal"
)

func seed(t *testing.T) (*Store, int64, int64, context.Context) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(pool.Close)
	_, _ = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var providerID, serviceID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1, 'AmazonEC2', 'EC2') RETURNING id`, providerID).Scan(&serviceID)
	var orgID int64
	pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('acme', 'acme') RETURNING id`).Scan(&orgID)
	return NewStore(pool), serviceID, orgID, ctx
}

func TestStore_AnonymousCreateReturnsEditToken(t *testing.T) {
	s, _, _, ctx := seed(t)
	est, edit, err := s.Create(ctx, CreateInput{Name: "anon", Currency: "USD"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if edit == "" || est.OrgID != nil {
		t.Errorf("est=%+v edit=%q", est, edit)
	}
	if !s.MatchesEditToken(ctx, est.ID, edit) {
		t.Error("MatchesEditToken false for own token")
	}
	if s.MatchesEditToken(ctx, est.ID, "wrong") {
		t.Error("MatchesEditToken true for wrong token")
	}
}

func TestStore_AuthedCreateHasNoEditToken(t *testing.T) {
	s, _, orgID, ctx := seed(t)
	est, edit, err := s.Create(ctx, CreateInput{Name: "authed", Currency: "USD", OrgID: &orgID})
	if err != nil || edit != "" {
		t.Errorf("edit=%q err=%v", edit, err)
	}
	if est.OrgID == nil || *est.OrgID != orgID {
		t.Errorf("org=%+v", est.OrgID)
	}
}

func TestStore_AddItemComputesMonthly(t *testing.T) {
	s, svcID, orgID, ctx := seed(t)
	est, _, _ := s.Create(ctx, CreateInput{Name: "x", Currency: "USD", OrgID: &orgID})
	item, err := s.AddItem(ctx, est.ID, AddItemInput{
		ServiceID: svcID, RegionCode: "us-east-1", SKU: "SK",
		Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(0.1),
		Unit: "Hrs", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	want := decimal.NewFromFloat(0.1).Mul(decimal.NewFromInt(730))
	if !item.MonthlyCostSnapshot.Equal(want) {
		t.Errorf("monthly = %s, want %s", item.MonthlyCostSnapshot, want)
	}
	got, _ := s.Get(ctx, est.ID)
	if len(got.Items) != 1 || !got.Total.Equal(want) {
		t.Errorf("got items=%d total=%s", len(got.Items), got.Total)
	}
}

func TestStore_ClaimByEditTokens(t *testing.T) {
	s, _, orgID, ctx := seed(t)
	_, edit, _ := s.Create(ctx, CreateInput{Name: "anon", Currency: "USD"})
	var userID int64
	s.pool.QueryRow(ctx, `INSERT INTO users (email) VALUES ('u@x.com') RETURNING id`).Scan(&userID)
	n, err := s.ClaimByEditTokens(ctx, orgID, userID, []string{edit})
	if err != nil || n != 1 {
		t.Errorf("n=%d err=%v", n, err)
	}
	if s.MatchesEditToken(ctx, 1, edit) {
		t.Error("edit token should have been cleared")
	}
}

func TestStore_ShareRoundTrip(t *testing.T) {
	s, _, orgID, ctx := seed(t)
	est, _, _ := s.Create(ctx, CreateInput{Name: "x", Currency: "USD", OrgID: &orgID})
	raw, err := s.CreateShare(ctx, est.ID)
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}
	got, err := s.LookupByShare(ctx, raw)
	if err != nil || got.ID != est.ID {
		t.Errorf("got=%+v err=%v", got, err)
	}
	if _, err := s.LookupByShare(ctx, "bogus"); err == nil {
		t.Error("expected error for bogus token")
	}
}

func TestStore_AddCuratedItemRoundTrip(t *testing.T) {
	s, svcID, orgID, ctx := seed(t)
	est, _, _ := s.Create(ctx, CreateInput{Name: "curated", Currency: "USD", OrgID: &orgID})

	input := map[string]any{
		"instanceType":  "m5.large",
		"operatingSystem": "Linux",
		"count":         float64(2),
		"hoursPerMonth": float64(730),
	}
	comps := []Component{
		{
			SKU: "EC2-COMPUTE-SKU", Unit: "Hrs",
			Quantity:    decimal.NewFromInt(1460),
			UnitPrice:   decimal.NewFromFloat(0.096),
			MonthlyCost: decimal.NewFromFloat(140.16),
			Currency:    "USD",
			Attributes:  map[string]string{"instanceType": "m5.large"},
			Region:      "us-east-1",
			Service:     "AmazonEC2",
		},
	}
	item, err := s.AddCuratedItem(ctx, est.ID, AddCuratedItemInput{
		ServiceID:         svcID,
		ServiceKey:        "ec2",
		RegionCode:        "us-east-1",
		SyntheticSKU:      "curated:ec2:v1",
		Currency:          "USD",
		UserInputSnapshot: input,
		Components:        comps,
	})
	if err != nil {
		t.Fatalf("AddCuratedItem: %v", err)
	}
	if item.LineType != LineTypeCurated || item.ServiceKey != "ec2" {
		t.Errorf("wrong line metadata: %+v", item)
	}
	if !item.MonthlyCostSnapshot.Equal(decimal.NewFromFloat(140.16)) {
		t.Errorf("monthly = %s, want 140.16", item.MonthlyCostSnapshot)
	}

	got, err := s.Get(ctx, est.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(got.Items))
	}
	back := got.Items[0]
	if back.LineType != LineTypeCurated {
		t.Errorf("LineType = %q", back.LineType)
	}
	if back.ServiceKey != "ec2" {
		t.Errorf("ServiceKey = %q", back.ServiceKey)
	}
	if len(back.ComponentsSnapshot) != 1 || back.ComponentsSnapshot[0].SKU != "EC2-COMPUTE-SKU" {
		t.Errorf("components lost: %+v", back.ComponentsSnapshot)
	}
	if back.UserInputSnapshot == nil {
		t.Errorf("user input snapshot missing")
	}
}
