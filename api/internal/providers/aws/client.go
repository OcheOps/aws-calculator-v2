package aws

import (
	"context"
	"encoding/json"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

// SDK is the minimal subset of the AWS Pricing API we depend on.
// Real implementation wraps the SDK; tests use a fake.
type SDK interface {
	ListServiceCodes(ctx context.Context) ([]string, error)
	DescribeService(ctx context.Context, code string) ([]string, error)
	GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error)
}

type Client struct {
	sdk SDK
}

func NewClient(sdk SDK) *Client { return &Client{sdk: sdk} }

func (c *Client) ListServices(ctx context.Context) ([]string, error) {
	return c.sdk.ListServiceCodes(ctx)
}

func (c *Client) DescribeService(ctx context.Context, code string) ([]string, error) {
	return c.sdk.DescribeService(ctx, code)
}

func (c *Client) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	return c.sdk.GetProducts(ctx, code, region, token)
}

// realSDK is the production implementation backed by aws-sdk-go-v2.
type realSDK struct {
	client *pricing.Client
}

func NewRealSDK(ctx context.Context, accessKeyID, secretAccessKey, region string) (SDK, error) {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &realSDK{client: pricing.NewFromConfig(cfg)}, nil
}

func (r *realSDK) ListServiceCodes(ctx context.Context) ([]string, error) {
	var out []string
	var token *string
	for {
		resp, err := r.client.DescribeServices(ctx, &pricing.DescribeServicesInput{
			NextToken:  token,
			MaxResults: awssdk.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("describe services: %w", err)
		}
		for _, s := range resp.Services {
			if s.ServiceCode != nil {
				out = append(out, *s.ServiceCode)
			}
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}
}

func (r *realSDK) DescribeService(ctx context.Context, code string) ([]string, error) {
	resp, err := r.client.DescribeServices(ctx, &pricing.DescribeServicesInput{
		ServiceCode: awssdk.String(code),
		MaxResults:  awssdk.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("describe service %s: %w", code, err)
	}
	if len(resp.Services) == 0 {
		return nil, fmt.Errorf("service %s not found", code)
	}
	return resp.Services[0].AttributeNames, nil
}

func (r *realSDK) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	in := &pricing.GetProductsInput{
		ServiceCode: awssdk.String(code),
		MaxResults:  awssdk.Int32(100),
		Filters: []types.Filter{{
			Field: awssdk.String("regionCode"),
			Type:  types.FilterTypeTermMatch,
			Value: awssdk.String(region),
		}},
	}
	if token != "" {
		in.NextToken = awssdk.String(token)
	}
	resp, err := r.client.GetProducts(ctx, in)
	if err != nil {
		return nil, "", fmt.Errorf("get products %s/%s: %w", code, region, err)
	}
	out := make([]json.RawMessage, 0, len(resp.PriceList))
	for _, s := range resp.PriceList {
		out = append(out, json.RawMessage(s))
	}
	next := ""
	if resp.NextToken != nil {
		next = *resp.NextToken
	}
	return out, next, nil
}
