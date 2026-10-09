package transfer_api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"

	fmeconfig "github.com/awslabs/filemoverexpress/config"
	"github.com/awslabs/filemoverexpress/core/auth"
	"github.com/awslabs/filemoverexpress/types/configtypes"
)

// sessionValidationTimeout bounds the STS check of a cached session so an unreachable STS endpoint cannot
// stall session creation indefinitely.
const sessionValidationTimeout = 30 * time.Second

var (
	configCache     = make(map[string]*aws.Config)
	configCacheLock = sync.RWMutex{}

	// loadConfigFunc and sessionValidatorFunc are package-level function variables
	// to allow tests to stub out AWS SDK calls without requiring real credentials.
	loadConfigFunc       = loadDefaultConfig
	loadStaticConfigFunc = loadStaticConfig
	sessionValidatorFunc = isSessionValid

	// oidcProvider is the package-level OIDC provider instance, set during daemon initialization.
	oidcProvider *auth.OIDCProvider

	errOIDCProviderNotInitialized = errors.New("OIDC provider not initialized")
)

type (
	S3ManagerConfig struct {
		AwsProfile string
		Bucket     string
		Region     string
		Endpoint   string
		Provider   configtypes.S3Provider
		PathStyle  bool
		TLS        configtypes.EndpointTLSSettings
	}

	//nolint:staticcheck // Pending TransferManagerV2 migration
	S3Manager struct {
		AwsProfile string
		Bucket     string
		Client     FileMoverS3ClientInterface
		Downloader *manager.Downloader
		Region     string
		Uploader   *manager.Uploader
		Lock       *sync.RWMutex
		// Provider is the normalized S3 provider; the zero value is treated as AWS.
		Provider configtypes.S3Provider
	}

	// oidcCredentialProvider implements aws.CredentialsProvider by delegating to the
	// OIDCProvider's GetCredentials method. The AWS SDK calls Retrieve() on each API
	// request (with internal caching based on CanExpire/Expires), enabling transparent
	// credential refresh during long-running transfers.
	oidcCredentialProvider struct {
		provider    *auth.OIDCProvider
		profileName string
		oidcConfig  *auth.OIDCConfig
	}
)

// SetOIDCProvider sets the package-level OIDC provider used for OIDC-authenticated sessions.
func SetOIDCProvider(provider *auth.OIDCProvider) {
	oidcProvider = provider
}

func isSessionValid(awsConfig aws.Config) bool {
	ctx, cancel := context.WithTimeout(context.Background(), sessionValidationTimeout)
	defer cancel()
	client := sts.NewFromConfig(awsConfig)
	_, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	return err == nil
}

// ValidateCredentials uses the S3 client to make a listObjectsV2 call to check if the credentials are valid. Returns an error if the
// credentials are invalid or if the user does not have permission to call ListObjectsV2.
func (s3m *S3Manager) ValidateCredentials() error {
	return s3m.ValidateCredentialsContext(context.TODO())
}

// ValidateCredentialsContext is ValidateCredentials with a caller-supplied context, so that interactive
// checks (e.g. the GUI's "Test connection") can time out.
func (s3m *S3Manager) ValidateCredentialsContext(ctx context.Context) error {
	maxKeys := int32(1)
	params := &s3.ListObjectsV2Input{
		Bucket:  &s3m.Bucket,
		Prefix:  aws.String("/"),
		MaxKeys: &maxKeys,
	}
	_, err := s3m.Client.ListObjectsV2(ctx, params)
	return err
}

// baseLoadOptions are the load options shared by every credential path: the region and the FME user agent.
func baseLoadOptions(region string) []func(*config.LoadOptions) error {
	return []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithAPIOptions([]func(stack *middleware.Stack) error{attachCustomMiddleware()}),
	}
}

func loadDefaultConfig(profile string, region string) (aws.Config, error) {
	return config.LoadDefaultConfig(
		context.TODO(),
		append(baseLoadOptions(region), config.WithSharedConfigProfile(profile))...,
	)
}

// loadStaticConfig builds a config from the access keys stored in the transfer profile. Static keys do not
// expire, so the result is neither cached nor validated with STS. The config is built directly rather than
// with LoadDefaultConfig, so that AWS_PROFILE and the shared AWS config files cannot interfere with a
// profile that has its own keys.
func loadStaticConfig(region string, creds configtypes.StaticCredentials) (aws.Config, error) {
	cfg := aws.NewConfig()
	cfg.Region = region
	cfg.Credentials = aws.NewCredentialsCache(
		credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken),
	)
	cfg.APIOptions = []func(*middleware.Stack) error{attachCustomMiddleware()}
	return *cfg, nil
}

// GetSession returns a cached shared-config session for an AWS named profile, re-validating cached entries
// with STS before reuse.
func GetSession(profile string, region string) (*aws.Config, error) {
	return loadCachedSession(profile, region, sessionValidatorFunc)
}

// sessionValidatorFor returns the cached-session validator for a provider. Non-AWS endpoints get none: STS
// is an AWS-only service, so the check can never succeed for their credentials and would defeat the cache.
func sessionValidatorFor(provider configtypes.S3Provider) func(aws.Config) bool {
	if provider.IsAWS() {
		return sessionValidatorFunc
	}
	return nil
}

// loadCachedSession is GetSession with a caller-supplied validator for cached entries; nil skips re-validation.
func loadCachedSession(profile string, region string, validator func(aws.Config) bool) (*aws.Config, error) {
	key := strings.Join([]string{region, profile}, "-")

	configCacheLock.RLock()
	existingCfg, entryExists := configCache[key]
	configCacheLock.RUnlock()

	if !entryExists || (validator != nil && !validator(*existingCfg)) {
		cfg, err := loadConfigFunc(profile, region)
		if err != nil {
			return nil, err
		}
		configCacheLock.Lock()
		configCache[key] = &cfg
		configCacheLock.Unlock()
	}

	configCacheLock.RLock()
	defer configCacheLock.RUnlock()
	return configCache[key], nil
}

// GetSessionForTransferProfile resolves an AWS config for the given transfer profile,
// routing to OIDC credential provider when auth_method is OIDC.
func GetSessionForTransferProfile(tp configtypes.TransferProfile) (*aws.Config, error) {
	if err := configtypes.ValidateProfileForProvider(tp); err != nil {
		return nil, err
	}

	switch tp.AuthMethod {
	case configtypes.AuthMethodOIDC:
		return getOIDCSession(tp)
	case configtypes.AuthMethodStaticKeys:
		// ValidateProfileForProvider guarantees StaticCredentials is set for this auth method.
		cfg, err := loadStaticConfigFunc(tp.Region, *tp.StaticCredentials)
		if err != nil {
			return nil, err
		}
		return &cfg, nil
	default:
		// AWS_PROFILE or UNSPECIFIED — use existing credential resolution
		return loadCachedSession(tp.Profile, tp.Region, sessionValidatorFor(tp.Provider))
	}
}

func (o *oidcCredentialProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	creds, err := o.provider.GetCredentials(o.profileName, o.oidcConfig)
	if err != nil {
		return aws.Credentials{}, err
	}
	return aws.Credentials{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Source:          "OIDCProvider",
		CanExpire:       true,
		Expires:         creds.Expiration,
	}, nil
}

func getOIDCSession(tp configtypes.TransferProfile) (*aws.Config, error) {
	if oidcProvider == nil {
		return nil, errOIDCProviderNotInitialized
	}

	oidcCfg := mapTransferProfileToOIDCConfig(tp.OIDCConfig)

	// Fail fast: verify credentials are obtainable before building the client.
	// This surfaces ErrOIDCNotAuthenticated immediately rather than on first S3 call.
	if _, err := oidcProvider.GetCredentials(tp.Name, oidcCfg); err != nil {
		return nil, err
	}

	provider := &oidcCredentialProvider{
		provider:    oidcProvider,
		profileName: tp.Name,
		oidcConfig:  oidcCfg,
	}

	cfg, err := buildOIDCConfig(tp.Region, provider)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// mapTransferProfileToOIDCConfig converts the config types to the auth package types.
func mapTransferProfileToOIDCConfig(src *configtypes.OIDCConfig) *auth.OIDCConfig {
	if src == nil {
		return nil
	}
	return &auth.OIDCConfig{
		IssuerURL:              src.IssuerURL,
		ClientID:               src.ClientID,
		RoleARN:                src.RoleARN,
		Scopes:                 src.Scopes,
		PersistSession:         src.PersistSession,
		CustomCABundle:         src.CustomCABundle,
		SessionDurationSeconds: src.SessionDurationSeconds,
	}
}

func buildOIDCConfig(region string, provider aws.CredentialsProvider) (aws.Config, error) {
	return config.LoadDefaultConfig(
		context.TODO(),
		append(baseLoadOptions(region), config.WithCredentialsProvider(provider))...,
	)
}

// NewS3Manager creates an S3Manager using the credential path appropriate for the transfer profile's auth method.
// For OIDC profiles, credentials are obtained from the OIDC provider; for AWS profile-based auth, standard credential
// resolution is used.
func NewS3Manager(tp configtypes.TransferProfile) (*S3Manager, error) {
	cfg, err := GetSessionForTransferProfile(tp)
	if err != nil {
		return nil, err
	}
	return buildS3Manager(cfg, tp)
}

// NewS3ManagerFromConfig creates an S3Manager using explicit AWS profile credentials.
// Use this only when a full TransferProfile is not available (e.g., CLI commands with manual args).
func NewS3ManagerFromConfig(input S3ManagerConfig) (*S3Manager, error) {
	return NewS3Manager(configtypes.TransferProfile{
		Profile:    input.AwsProfile,
		Bucket:     input.Bucket,
		Region:     input.Region,
		Endpoint:   input.Endpoint,
		Provider:   input.Provider,
		PathStyle:  input.PathStyle,
		TLS:        input.TLS,
		AuthMethod: configtypes.AuthMethodAWSProfile,
	})
}

func buildS3Manager(cfg *aws.Config, tp configtypes.TransferProfile) (*S3Manager, error) {
	retryCount := fmeconfig.LoadConfiguration().General.RetryCount
	retryCount = max(retryCount, 0)
	optFn, err := s3ClientOptions(tp, retryCount)
	if err != nil {
		return nil, err
	}
	return &S3Manager{
		Region:     tp.Region,
		Bucket:     tp.Bucket,
		AwsProfile: tp.Profile,
		Provider:   tp.Provider.Normalized(),
		Client: &FileMoverS3Client{
			client: s3.NewFromConfig(*cfg, optFn),
		},
		Lock: &sync.RWMutex{},
	}, nil
}

// s3ClientOptions builds the per-client option function for a transfer profile. Non-AWS endpoints get
// path-style addressing when requested and relaxed checksum handling, because many S3-compatible services
// reject the CRC32 request trailers and response validation the SDK enables by default.
func s3ClientOptions(tp configtypes.TransferProfile, retryCount uint32) (func(*s3.Options), error) {
	httpClient, err := buildEndpointHTTPClient(tp.TLS, tp.Name)
	if err != nil {
		return nil, err
	}
	isAWS := tp.Provider.IsAWS()

	return func(opts *s3.Options) {
		opts.RetryMaxAttempts = int(retryCount)
		if tp.Endpoint != "" {
			opts.BaseEndpoint = aws.String(tp.Endpoint)
		}
		if httpClient != nil {
			opts.HTTPClient = httpClient
		}
		if isAWS {
			return
		}
		opts.UsePathStyle = tp.PathStyle
		opts.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		opts.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}, nil
}
