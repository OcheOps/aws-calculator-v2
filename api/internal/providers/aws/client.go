package aws

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
)

// PriceListRef identifies a downloadable AWS Pricing price-list file for a
// single service+region.
type PriceListRef struct {
	ARN       string
	Region    string
	Currency  string
	Effective time.Time
	Formats   []string
}

// SDK is the minimal subset of the AWS Pricing API we depend on.
// Real implementation wraps the SDK; tests use a fake.
type SDK interface {
	ListServiceCodes(ctx context.Context) ([]string, error)
	DescribeService(ctx context.Context, code string) ([]string, error)
	ListPriceLists(ctx context.Context, serviceCode string, effective time.Time) ([]PriceListRef, error)
	GetPriceListFileURL(ctx context.Context, arn, format string) (string, error)
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


func (r *realSDK) ListPriceLists(ctx context.Context, serviceCode string, effective time.Time) ([]PriceListRef, error) {
	var out []PriceListRef
	var token *string
	for {
		resp, err := r.client.ListPriceLists(ctx, &pricing.ListPriceListsInput{
			ServiceCode:   awssdk.String(serviceCode),
			EffectiveDate: awssdk.Time(effective),
			CurrencyCode:  awssdk.String("USD"),
			NextToken:     token,
			MaxResults:    awssdk.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("list price lists %s: %w", serviceCode, err)
		}
		for _, pl := range resp.PriceLists {
			ref := PriceListRef{}
			if pl.PriceListArn != nil {
				ref.ARN = *pl.PriceListArn
			}
			if pl.RegionCode != nil {
				ref.Region = *pl.RegionCode
			}
			if pl.CurrencyCode != nil {
				ref.Currency = *pl.CurrencyCode
			}
			ref.Formats = append(ref.Formats, pl.FileFormats...)
			out = append(out, ref)
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}
}

func (r *realSDK) GetPriceListFileURL(ctx context.Context, arn, format string) (string, error) {
	resp, err := r.client.GetPriceListFileUrl(ctx, &pricing.GetPriceListFileUrlInput{
		PriceListArn: awssdk.String(arn),
		FileFormat:   awssdk.String(format),
	})
	if err != nil {
		return "", fmt.Errorf("get price list file url %s: %w", arn, err)
	}
	if resp.Url == nil {
		return "", fmt.Errorf("empty url for %s", arn)
	}
	return *resp.Url, nil
}

// Client passthrough methods so callers don't need to touch the SDK directly.

func (c *Client) ListPriceLists(ctx context.Context, serviceCode string, effective time.Time) ([]PriceListRef, error) {
	return c.sdk.ListPriceLists(ctx, serviceCode, effective)
}

func (c *Client) GetPriceListFileURL(ctx context.Context, arn, format string) (string, error) {
	return c.sdk.GetPriceListFileURL(ctx, arn, format)
}
