package transfer_api

import (
	"errors"
	"net/http"
	"sync"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/awslabs/filemoverexpress/types/configtypes"
)

func httpResponseError(status int) error {
	return &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}},
			Err:      errors.New("boom"),
		},
	}
}

func TestIsUnsupportedOperation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "NotImplemented api error", err: &smithy.GenericAPIError{Code: "NotImplemented"}, want: true},
		{name: "MethodNotAllowed api error", err: &smithy.GenericAPIError{Code: "MethodNotAllowed"}, want: true},
		{name: "NotSupported api error", err: &smithy.GenericAPIError{Code: "NotSupported"}, want: true},
		{name: "AccessDenied api error", err: &smithy.GenericAPIError{Code: "AccessDenied"}, want: false},
		{name: "http 501", err: httpResponseError(http.StatusNotImplemented), want: true},
		{name: "http 405", err: httpResponseError(http.StatusMethodNotAllowed), want: true},
		{name: "http 404", err: httpResponseError(http.StatusNotFound), want: false},
		{name: "plain error", err: errors.New("network down"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnsupportedOperation(tt.err); got != tt.want {
				t.Errorf("isUnsupportedOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorageClassFor(t *testing.T) {
	tests := []struct {
		name     string
		provider configtypes.S3Provider
		input    string
		want     types.StorageClass
	}{
		{name: "aws standard", provider: configtypes.S3ProviderAWS, input: "standard", want: types.StorageClassStandard},
		{name: "legacy aws glacier", provider: configtypes.S3ProviderUnspecified, input: "glacier", want: types.StorageClassGlacier},
		{name: "aws unknown falls back to standard", provider: configtypes.S3ProviderAWS, input: "weird", want: types.StorageClassStandard},
		{name: "minio standard omitted", provider: configtypes.S3ProviderMinIO, input: "standard", want: ""},
		{name: "minio empty omitted", provider: configtypes.S3ProviderMinIO, input: "", want: ""},
		{name: "minio listed STANDARD omitted", provider: configtypes.S3ProviderMinIO, input: "STANDARD", want: ""},
		{name: "minio custom class passed through", provider: configtypes.S3ProviderMinIO, input: "reduced redundancy", want: "REDUCED_REDUNDANCY"},
		{name: "backblaze never sends a class", provider: configtypes.S3ProviderBackblaze, input: "standard_ia", want: ""},
		{name: "hetzner never sends a class", provider: configtypes.S3ProviderHetzner, input: "standard_ia", want: ""},
		{name: "r2 standard omitted", provider: configtypes.S3ProviderCloudflare, input: "STANDARD", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3m := &S3Manager{Provider: tt.provider, Lock: &sync.RWMutex{}}
			got, err := s3m.storageClassFor(tt.input)
			if err != nil {
				t.Fatalf("storageClassFor() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("storageClassFor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsBatchDeleteRejected(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "MissingContentMD5", err: &smithy.GenericAPIError{Code: "MissingContentMD5"}, want: true},
		{name: "InvalidDigest", err: &smithy.GenericAPIError{Code: "InvalidDigest"}, want: true},
		{name: "NotImplemented", err: &smithy.GenericAPIError{Code: "NotImplemented"}, want: true},
		{name: "http 405 without code", err: httpResponseError(http.StatusMethodNotAllowed), want: true},
		{name: "AccessDenied", err: &smithy.GenericAPIError{Code: "AccessDenied"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBatchDeleteRejected(tt.err); got != tt.want {
				t.Errorf("isBatchDeleteRejected() = %v, want %v", got, tt.want)
			}
		})
	}
}
