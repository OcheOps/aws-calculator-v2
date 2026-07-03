package aws

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeSDK struct {
	services  []string
	products  map[string][]string // key: serviceCode|region → JSON payloads
	nextToken string
}

func (f *fakeSDK) ListServiceCodes(ctx context.Context) ([]string, error) {
	return f.services, nil
}

func (f *fakeSDK) DescribeService(ctx context.Context, code string) ([]string, error) {
	return []string{"instanceType", "regionCode"}, nil
}

func (f *fakeSDK) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	key := code + "|" + region
	payloads := f.products[key]
	out := make([]json.RawMessage, 0, len(payloads))
	for _, p := range payloads {
		out = append(out, json.RawMessage(p))
	}
	return out, "", nil
}

func TestClient_ListServices(t *testing.T) {
	c := NewClient(&fakeSDK{services: []string{"AmazonEC2", "AmazonS3"}})
	ctx := context.Background()
	svcs, err := c.ListServices(ctx)
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != 2 || svcs[0] != "AmazonEC2" {
		t.Errorf("services = %v", svcs)
	}
}
