package transfer_api

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/awslabs/filemoverexpress/core/transfer-api/mock"
	"github.com/awslabs/filemoverexpress/types/configtypes"
)

func restoreSessionStubs(t *testing.T) {
	t.Helper()
	origLoad := loadConfigFunc
	origStatic := loadStaticConfigFunc
	origValidator := sessionValidatorFunc
	t.Cleanup(func() {
		loadConfigFunc = origLoad
		loadStaticConfigFunc = origStatic
		sessionValidatorFunc = origValidator
		configCacheLock.Lock()
		configCache = make(map[string]*aws.Config)
		configCacheLock.Unlock()
	})
}

func TestS3ClientOptions_PerProvider(t *testing.T) {
	tests := []struct {
		name              string
		profile           configtypes.TransferProfile
		wantPathStyle     bool
		wantWhenRequired  bool
		wantBaseEndpoint  string
		wantRetryAttempts int
	}{
		{
			name:              "legacy aws profile keeps SDK defaults",
			profile:           configtypes.TransferProfile{Bucket: "b", Region: "us-east-1", PathStyle: true},
			wantRetryAttempts: 3,
		},
		{
			name:             "aws with vpc endpoint keeps base endpoint only",
			profile:          configtypes.TransferProfile{Provider: configtypes.S3ProviderAWS, Endpoint: "https://vpce.example"},
			wantBaseEndpoint: "https://vpce.example",
		},
		{
			name: "minio uses path style and relaxed checksums",
			profile: configtypes.TransferProfile{
				Provider: configtypes.S3ProviderMinIO, Endpoint: "http://127.0.0.1:9000", PathStyle: true,
			},
			wantPathStyle:    true,
			wantWhenRequired: true,
			wantBaseEndpoint: "http://127.0.0.1:9000",
		},
		{
			name: "r2 honours virtual-hosted style but still relaxes checksums",
			profile: configtypes.TransferProfile{
				Provider: configtypes.S3ProviderCloudflare, Endpoint: "https://acct.r2.cloudflarestorage.com",
			},
			wantWhenRequired: true,
			wantBaseEndpoint: "https://acct.r2.cloudflarestorage.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optFn, err := s3ClientOptions(tt.profile, uint32(tt.wantRetryAttempts))
			if err != nil {
				t.Fatalf("s3ClientOptions() error = %v", err)
			}
			opts := &s3.Options{}
			optFn(opts)

			if opts.UsePathStyle != tt.wantPathStyle {
				t.Errorf("UsePathStyle = %v, want %v", opts.UsePathStyle, tt.wantPathStyle)
			}
			gotWhenRequired := opts.RequestChecksumCalculation == aws.RequestChecksumCalculationWhenRequired &&
				opts.ResponseChecksumValidation == aws.ResponseChecksumValidationWhenRequired
			if gotWhenRequired != tt.wantWhenRequired {
				t.Errorf("checksum WhenRequired = %v, want %v", gotWhenRequired, tt.wantWhenRequired)
			}
			if aws.ToString(opts.BaseEndpoint) != tt.wantBaseEndpoint {
				t.Errorf("BaseEndpoint = %q, want %q", aws.ToString(opts.BaseEndpoint), tt.wantBaseEndpoint)
			}
			if opts.RetryMaxAttempts != tt.wantRetryAttempts {
				t.Errorf("RetryMaxAttempts = %d, want %d", opts.RetryMaxAttempts, tt.wantRetryAttempts)
			}
			if opts.HTTPClient != nil {
				t.Errorf("HTTPClient should be nil when no TLS settings are configured")
			}
		})
	}
}

func TestS3ClientOptions_InvalidCABundle(t *testing.T) {
	profile := configtypes.TransferProfile{
		Provider: configtypes.S3ProviderOther,
		Endpoint: "https://s3.local",
		TLS:      configtypes.EndpointTLSSettings{CABundle: "/does/not/exist.pem"},
	}
	if _, err := s3ClientOptions(profile, 1); err == nil {
		t.Fatal("s3ClientOptions() expected an error for a missing CA bundle")
	}
}

func TestGetSessionForTransferProfile_SkipsSTSValidationForNonAWS(t *testing.T) {
	restoreSessionStubs(t)
	loadConfigFunc = stubLoadConfig
	validatorCalls := 0
	sessionValidatorFunc = func(aws.Config) bool {
		validatorCalls++
		return false // force reload whenever validation runs
	}

	minio := configtypes.TransferProfile{
		Name: "minio", Provider: configtypes.S3ProviderMinIO, Endpoint: "http://127.0.0.1:9000",
		Profile: "minio", Region: "us-east-1", AuthMethod: configtypes.AuthMethodAWSProfile,
	}
	for range 2 {
		if _, err := GetSessionForTransferProfile(minio); err != nil {
			t.Fatalf("GetSessionForTransferProfile(minio) error = %v", err)
		}
	}
	if validatorCalls != 0 {
		t.Errorf("STS validator called %d times for a non-AWS profile, want 0", validatorCalls)
	}

	awsProfile := configtypes.TransferProfile{Name: "aws", Profile: "default", Region: mock.UnitTestMockRegion}
	for range 2 {
		if _, err := GetSessionForTransferProfile(awsProfile); err != nil {
			t.Fatalf("GetSessionForTransferProfile(aws) error = %v", err)
		}
	}
	if validatorCalls == 0 {
		t.Errorf("STS validator should run for cached AWS sessions")
	}
}

func TestGetSessionForTransferProfile_StaticKeys(t *testing.T) {
	restoreSessionStubs(t)
	loadConfigFunc = func(string, string) (aws.Config, error) {
		t.Fatal("named-profile loader must not be used for static keys")
		return aws.Config{}, nil
	}

	tp := configtypes.TransferProfile{
		Name:       "keys",
		Provider:   configtypes.S3ProviderOther,
		Endpoint:   "https://s3.local",
		Region:     "us-east-1",
		AuthMethod: configtypes.AuthMethodStaticKeys,
		StaticCredentials: &configtypes.StaticCredentials{
			AccessKeyID: "AKIATEST", SecretAccessKey: "secret", SessionToken: "token",
		},
	}
	cfg, err := GetSessionForTransferProfile(tp)
	if err != nil {
		t.Fatalf("GetSessionForTransferProfile() error = %v", err)
	}
	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if creds.AccessKeyID != "AKIATEST" || creds.SecretAccessKey != "secret" || creds.SessionToken != "token" {
		t.Errorf("unexpected credentials %+v", creds)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("Region = %q, want us-east-1", cfg.Region)
	}
}

func TestGetSessionForTransferProfile_RejectsInvalidProviderCombinations(t *testing.T) {
	restoreSessionStubs(t)
	loadConfigFunc = stubLoadConfig
	sessionValidatorFunc = stubSessionValidator

	tests := []struct {
		name    string
		profile configtypes.TransferProfile
		wantErr error
	}{
		{
			name:    "oidc on minio",
			profile: configtypes.TransferProfile{Provider: configtypes.S3ProviderMinIO, Endpoint: "https://m", AuthMethod: configtypes.AuthMethodOIDC},
			wantErr: configtypes.ErrOIDCUnsupported,
		},
		{
			name:    "non-aws without endpoint",
			profile: configtypes.TransferProfile{Provider: configtypes.S3ProviderWasabi, Profile: "w"},
			wantErr: configtypes.ErrEndpointRequired,
		},
		{
			name:    "static keys without secret",
			profile: configtypes.TransferProfile{AuthMethod: configtypes.AuthMethodStaticKeys, StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "A"}},
			wantErr: configtypes.ErrStaticKeysIncomplete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetSessionForTransferProfile(tt.profile)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewS3Manager_SetsNormalizedProvider(t *testing.T) {
	restoreSessionStubs(t)
	loadConfigFunc = stubLoadConfig
	sessionValidatorFunc = stubSessionValidator

	legacy, err := NewS3Manager(configtypes.TransferProfile{Bucket: "b", Region: mock.UnitTestMockRegion})
	if err != nil {
		t.Fatalf("NewS3Manager(legacy) error = %v", err)
	}
	if legacy.Provider != configtypes.S3ProviderAWS {
		t.Errorf("legacy Provider = %q, want aws", legacy.Provider)
	}

	minio, err := NewS3ManagerFromConfig(S3ManagerConfig{
		AwsProfile: "minio", Bucket: "b", Region: "us-east-1", Endpoint: "http://127.0.0.1:9000",
		Provider: configtypes.S3ProviderMinIO, PathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3ManagerFromConfig(minio) error = %v", err)
	}
	if minio.Provider != configtypes.S3ProviderMinIO {
		t.Errorf("minio Provider = %q, want minio", minio.Provider)
	}
}

func TestLoadStaticConfig_IgnoresSharedConfigProfile(t *testing.T) {
	// A missing named profile in the environment must not break a profile that carries its own keys.
	t.Setenv("AWS_PROFILE", "does-not-exist")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))

	cfg, err := loadStaticConfig("us-east-1", configtypes.StaticCredentials{AccessKeyID: "AKIATEST", SecretAccessKey: "secret"})
	if err != nil {
		t.Fatalf("loadStaticConfig() error = %v", err)
	}
	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if creds.AccessKeyID != "AKIATEST" || creds.SecretAccessKey != "secret" {
		t.Errorf("unexpected credentials %+v", creds)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("Region = %q, want us-east-1", cfg.Region)
	}
	if len(cfg.APIOptions) == 0 {
		t.Error("the FME user-agent middleware should be attached")
	}
}

func TestNewS3ManagerFromConfig_ValidatesProvider(t *testing.T) {
	restoreSessionStubs(t)
	_, err := NewS3ManagerFromConfig(S3ManagerConfig{Bucket: "b", Region: "us-east-1", Provider: configtypes.S3ProviderMinIO})
	if !errors.Is(err, configtypes.ErrEndpointRequired) {
		t.Errorf("NewS3ManagerFromConfig() error = %v, want ErrEndpointRequired", err)
	}
}
