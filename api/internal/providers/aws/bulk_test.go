package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func servePriceList(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "price_list_ec2.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "\"ec2-fixture-v1\"")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func TestBulkClient_DownloadAndStream_EmitsRows(t *testing.T) {
	srv := servePriceList(t)
	defer srv.Close()

	b := NewBulkClient(nil) // downloader doesn't need the SDK
	out := make(chan SKURow, 8)
	etag, err := b.DownloadAndStream(context.Background(), srv.URL, "us-east-1", out)
	close(out)
	if err != nil {
		t.Fatalf("DownloadAndStream: %v", err)
	}
	if etag != "\"ec2-fixture-v1\"" {
		t.Errorf("etag = %q", etag)
	}

	rows := make(map[string]SKURow)
	for r := range out {
		rows[r.SKU] = r
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	m5 := rows["SKU-M5-LARGE-USE1"]
	if m5.Price != "0.096" || m5.Unit != "Hrs" || m5.Currency != "USD" || m5.TermType != "OnDemand" {
		t.Errorf("m5.large = %+v", m5)
	}
	if m5.Attributes["instanceType"] != "m5.large" {
		t.Errorf("attributes lost: %+v", m5.Attributes)
	}
	if rows["SKU-M5-XL-USE1"].Price != "0.192" {
		t.Errorf("xlarge price = %+v", rows["SKU-M5-XL-USE1"])
	}
}

func TestBulkClient_HeadETag(t *testing.T) {
	srv := servePriceList(t)
	defer srv.Close()

	b := NewBulkClient(nil)
	etag, err := b.HeadETag(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("HeadETag: %v", err)
	}
	if etag != "\"ec2-fixture-v1\"" {
		t.Errorf("etag = %q", etag)
	}
}
