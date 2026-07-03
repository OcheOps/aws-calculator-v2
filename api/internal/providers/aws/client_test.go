package aws

import (
	"context"
	"testing"
	"time"
)

type fakeSDK struct {
	services []string
}

func (f *fakeSDK) ListServiceCodes(ctx context.Context) ([]string, error) {
	return f.services, nil
}

func (f *fakeSDK) DescribeService(ctx context.Context, code string) ([]string, error) {
	return []string{"instanceType", "regionCode"}, nil
}

func (f *fakeSDK) ListPriceLists(ctx context.Context, serviceCode string, effective time.Time) ([]PriceListRef, error) {
	return nil, nil
}

func (f *fakeSDK) GetPriceListFileURL(ctx context.Context, arn, format string) (string, error) {
	return "", nil
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
