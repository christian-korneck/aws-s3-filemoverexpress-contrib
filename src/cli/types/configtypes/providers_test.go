package configtypes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awslabs/filemoverexpress/types/pbtypes/fme/v1"
)

func TestS3Provider_Normalized(t *testing.T) {
	assert.Equal(t, S3ProviderAWS, S3ProviderUnspecified.Normalized())
	assert.Equal(t, S3ProviderMinIO, S3ProviderMinIO.Normalized())
	assert.Equal(t, S3ProviderOther, S3Provider("bogus").Normalized())

	assert.True(t, S3ProviderUnspecified.IsKnown())
	assert.True(t, S3ProviderCloudflare.IsKnown())
	assert.False(t, S3Provider("bogus").IsKnown())

	assert.True(t, S3ProviderUnspecified.IsAWS())
	assert.True(t, S3ProviderAWS.IsAWS())
	assert.False(t, S3ProviderMinIO.IsAWS())
}

func TestS3Provider_SpecTable(t *testing.T) {
	assert.Equal(t, "us-east-2", S3ProviderAWS.Spec().DefaultRegion)
	assert.Equal(t, "auto", S3ProviderCloudflare.Spec().DefaultRegion)
	assert.False(t, S3ProviderCloudflare.Spec().SupportsVersioning)
	assert.True(t, S3ProviderMinIO.Spec().DefaultPathStyle)
	assert.False(t, S3ProviderBackblaze.Spec().SupportsStorageClass)
	assert.True(t, S3ProviderAWS.Spec().SupportsOIDC)
	assert.False(t, S3ProviderOther.Spec().SupportsOIDC)
	assert.Equal(t, "fsn1", S3ProviderHetzner.Spec().DefaultRegion)
	assert.Equal(t, "https://{region}.your-objectstorage.com", S3ProviderHetzner.Spec().EndpointTemplate)
	assert.False(t, S3ProviderHetzner.Spec().DefaultPathStyle)
	assert.True(t, S3ProviderHetzner.Spec().SupportsVersioning)
	assert.False(t, S3ProviderHetzner.Spec().SupportsStorageClass)
	assert.False(t, S3ProviderHetzner.Spec().SupportsOIDC)
	// Unknown providers behave like the generic preset.
	assert.Equal(t, S3ProviderOther.Spec(), S3Provider("bogus").Spec())
}

func TestS3Provider_ProtobufRoundTrip(t *testing.T) {
	for provider := range providerSpecs {
		assert.Equal(t, provider, S3ProviderFromProtobuf(provider.ToProtobuf()), "provider %q", provider)
	}
	// The legacy empty value must stay empty so saved files do not gain a provider key.
	assert.Equal(t, S3ProviderUnspecified, S3ProviderFromProtobuf(fmev1.S3Provider_S3_PROVIDER_UNSPECIFIED))
	assert.Equal(t, fmev1.S3Provider_S3_PROVIDER_UNSPECIFIED, S3ProviderUnspecified.ToProtobuf())
	assert.Equal(t, fmev1.S3Provider_S3_PROVIDER_AWS, S3ProviderAWS.ToProtobuf())
	assert.Equal(t, fmev1.S3Provider_S3_PROVIDER_OTHER, S3Provider("bogus").ToProtobuf())
}

func TestValidateProfileForProvider(t *testing.T) {
	creds := &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "secret"}
	tests := []struct {
		name    string
		profile TransferProfile
		wantErr error
	}{
		{name: "legacy aws profile without endpoint", profile: TransferProfile{Bucket: "b", Region: "us-east-1"}},
		{name: "aws with oidc", profile: TransferProfile{Provider: S3ProviderAWS, AuthMethod: AuthMethodOIDC}},
		{name: "aws with static keys", profile: TransferProfile{Provider: S3ProviderAWS, AuthMethod: AuthMethodStaticKeys, StaticCredentials: creds}},
		{
			name:    "minio without endpoint",
			profile: TransferProfile{Provider: S3ProviderMinIO, AuthMethod: AuthMethodStaticKeys, StaticCredentials: creds},
			wantErr: ErrEndpointRequired,
		},
		{
			name:    "minio with endpoint missing scheme",
			profile: TransferProfile{Provider: S3ProviderMinIO, Endpoint: "minio.local:9000", StaticCredentials: creds, AuthMethod: AuthMethodStaticKeys},
			wantErr: ErrEndpointInvalid,
		},
		{
			name:    "minio with ftp scheme",
			profile: TransferProfile{Provider: S3ProviderMinIO, Endpoint: "ftp://minio.local", StaticCredentials: creds, AuthMethod: AuthMethodStaticKeys},
			wantErr: ErrEndpointInvalid,
		},
		{
			name:    "minio valid http endpoint and static keys",
			profile: TransferProfile{Provider: S3ProviderMinIO, Endpoint: "http://127.0.0.1:9000", StaticCredentials: creds, AuthMethod: AuthMethodStaticKeys},
		},
		{
			name:    "minio with named profile",
			profile: TransferProfile{Provider: S3ProviderMinIO, Endpoint: "https://minio.local", Profile: "minio", AuthMethod: AuthMethodAWSProfile},
		},
		{
			name:    "minio with oidc rejected",
			profile: TransferProfile{Provider: S3ProviderMinIO, Endpoint: "https://minio.local", AuthMethod: AuthMethodOIDC},
			wantErr: ErrOIDCUnsupported,
		},
		{
			name:    "static keys without credentials block",
			profile: TransferProfile{Provider: S3ProviderOther, Endpoint: "https://s3.local", AuthMethod: AuthMethodStaticKeys},
			wantErr: ErrStaticKeysIncomplete,
		},
		{
			name: "static keys without secret",
			profile: TransferProfile{
				Provider: S3ProviderOther, Endpoint: "https://s3.local", AuthMethod: AuthMethodStaticKeys,
				StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA"},
			},
			wantErr: ErrStaticKeysIncomplete,
		},
		{
			name:    "unknown provider is treated as other",
			profile: TransferProfile{Provider: "bogus", Endpoint: "https://s3.local"},
		},
		{
			name:    "unknown provider still needs an endpoint",
			profile: TransferProfile{Provider: "bogus"},
			wantErr: ErrEndpointRequired,
		},
		{
			name:    "endpoint with template placeholder rejected",
			profile: TransferProfile{Provider: S3ProviderCloudflare, Endpoint: "https://{account_id}.r2.cloudflarestorage.com"},
			wantErr: ErrEndpointInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileForProvider(tt.profile)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestS3Provider_CaseInsensitive(t *testing.T) {
	for _, value := range []S3Provider{"MinIO", " minio ", "MINIO"} {
		assert.True(t, value.IsKnown(), value)
		assert.Equal(t, S3ProviderMinIO, value.Normalized(), value)
		assert.Equal(t, fmev1.S3Provider_S3_PROVIDER_MINIO, value.ToProtobuf(), value)
	}
	assert.False(t, S3Provider("bogus").IsKnown())
	assert.Equal(t, S3ProviderOther, S3Provider("bogus").Normalized())
	assert.Equal(t, S3ProviderAWS, S3Provider("  ").Normalized())
}

func TestTransferProfile_ConnectionKeyChangesWithSecret(t *testing.T) {
	base := TransferProfile{
		Name: "p", Provider: S3ProviderMinIO, Endpoint: "https://a", AuthMethod: AuthMethodStaticKeys,
		StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "old", SessionToken: "token"},
	}
	rotated := base
	rotated.StaticCredentials = &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "new", SessionToken: "token"}
	newToken := base
	newToken.StaticCredentials = &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "old", SessionToken: "token2"}

	assert.NotEqual(t, base.ConnectionKey(), rotated.ConnectionKey())
	assert.NotEqual(t, base.ConnectionKey(), newToken.ConnectionKey())
	assert.NotContains(t, base.ConnectionKey(), "old")
	assert.NotContains(t, base.ConnectionKey(), "token")
}

func TestTransferProfile_RestoreRedactedSecrets(t *testing.T) {
	stored := map[string]TransferProfile{
		"minio": {Name: "minio", StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "s3cr3t", SessionToken: "tok"}},
	}

	t.Run("placeholders are replaced by stored values", func(t *testing.T) {
		tp := TransferProfile{Name: "minio", StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: RedactedSecret, SessionToken: RedactedSecret}}
		assert.NoError(t, tp.RestoreRedactedSecrets(stored))
		assert.Equal(t, "s3cr3t", tp.StaticCredentials.SecretAccessKey)
		assert.Equal(t, "tok", tp.StaticCredentials.SessionToken)
		assert.NotSame(t, stored["minio"].StaticCredentials, tp.StaticCredentials, "stored credentials must not be shared")
	})
	t.Run("new secret and cleared token are kept", func(t *testing.T) {
		tp := TransferProfile{Name: "minio", StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "fresh", SessionToken: RedactedSecret}}
		assert.NoError(t, tp.RestoreRedactedSecrets(stored))
		assert.Equal(t, "fresh", tp.StaticCredentials.SecretAccessKey)
		assert.Equal(t, "tok", tp.StaticCredentials.SessionToken)

		cleared := TransferProfile{Name: "minio", StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: RedactedSecret}}
		assert.NoError(t, cleared.RestoreRedactedSecrets(stored))
		assert.Empty(t, cleared.StaticCredentials.SessionToken)
	})
	t.Run("changed access key id requires the secret again", func(t *testing.T) {
		tp := TransferProfile{Name: "minio", StaticCredentials: &StaticCredentials{AccessKeyID: "OTHER", SecretAccessKey: RedactedSecret}}
		assert.ErrorIs(t, tp.RestoreRedactedSecrets(stored), ErrSecretMustBeReentered)
	})
	t.Run("renamed profile requires the secret again", func(t *testing.T) {
		tp := TransferProfile{Name: "renamed", StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: RedactedSecret}}
		assert.ErrorIs(t, tp.RestoreRedactedSecrets(stored), ErrSecretMustBeReentered)
	})
	t.Run("profiles without placeholders are untouched", func(t *testing.T) {
		tp := TransferProfile{Name: "x"}
		assert.NoError(t, tp.RestoreRedactedSecrets(nil))
		assert.Nil(t, tp.StaticCredentials)
	})
}

func TestFmeConfig_RedactedHidesPreSharedKey(t *testing.T) {
	cfg := FmeConfig{}
	cfg.APIServer.RemoteSettings.PreSharedKey = "psk-value"
	assert.Equal(t, RedactedSecret, cfg.Redacted().APIServer.RemoteSettings.PreSharedKey)
	assert.Equal(t, "psk-value", cfg.APIServer.RemoteSettings.PreSharedKey)
}

func TestTransferProfileFromProtobuf_DropsCredentialsForOtherAuthMethods(t *testing.T) {
	creds := &fmev1.StaticCredentials{AccessKeyId: "AKIA", SecretAccessKey: "s3cr3t"}
	profile := TransferProfileFromProtobuf(&fmev1.TransferProfile{AuthMethod: fmev1.AuthMethod_AUTH_METHOD_AWS_PROFILE, StaticCredentials: creds})
	assert.Nil(t, profile.StaticCredentials)

	profile = TransferProfileFromProtobuf(&fmev1.TransferProfile{AuthMethod: fmev1.AuthMethod_AUTH_METHOD_STATIC_KEYS, StaticCredentials: creds})
	assert.Equal(t, "s3cr3t", profile.StaticCredentials.SecretAccessKey)
}

func TestTransferProfile_ConnectionKey(t *testing.T) {
	base := TransferProfile{
		Name: "p", Provider: S3ProviderMinIO, Bucket: "b", Region: "us-east-1", Endpoint: "https://a",
		AuthMethod: AuthMethodStaticKeys, StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "s3cr3t"},
	}
	assert.NotContains(t, base.ConnectionKey(), "s3cr3t")
	assert.Contains(t, base.ConnectionKey(), "AKIA")

	pathStyle := base
	pathStyle.PathStyle = true
	assert.NotEqual(t, base.ConnectionKey(), pathStyle.ConnectionKey())

	otherEndpoint := base
	otherEndpoint.Endpoint = "https://b"
	assert.NotEqual(t, base.ConnectionKey(), otherEndpoint.ConnectionKey())

	skipVerify := base
	skipVerify.TLS.SkipVerify = true
	assert.NotEqual(t, base.ConnectionKey(), skipVerify.ConnectionKey())

	// Legacy and explicit AWS resolve to the same connection.
	legacy := TransferProfile{Name: "p", Bucket: "b", Region: "us-east-1"}
	explicit := legacy
	explicit.Provider = S3ProviderAWS
	assert.Equal(t, legacy.ConnectionKey(), explicit.ConnectionKey())
}

func TestStaticCredentials_Redaction(t *testing.T) {
	var nilCreds *StaticCredentials
	assert.Equal(t, "<nil>", nilCreds.String())
	assert.Nil(t, nilCreds.Redacted())

	creds := &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "s3cr3t", SessionToken: "t0ken"}
	for _, rendered := range []string{creds.String(), creds.GoString(), fmt.Sprintf("%v", creds), fmt.Sprintf("%+v", creds)} {
		assert.NotContains(t, rendered, "s3cr3t")
		assert.NotContains(t, rendered, "t0ken")
		assert.Contains(t, rendered, "AKIA")
	}

	redacted := creds.Redacted()
	assert.Equal(t, "AKIA", redacted.AccessKeyID)
	assert.Equal(t, RedactedSecret, redacted.SecretAccessKey)
	assert.Equal(t, RedactedSecret, redacted.SessionToken)
	// The original is untouched.
	assert.Equal(t, "s3cr3t", creds.SecretAccessKey)

	// Empty optional values stay empty instead of becoming the placeholder.
	assert.Empty(t, (&StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "x"}).Redacted().SessionToken)
}

func TestTransferProfile_String_RedactsSecret(t *testing.T) {
	tp := TransferProfile{
		Name:              "p",
		Bucket:            "bucket",
		StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "s3cr3t"},
	}
	out := tp.String()
	assert.Contains(t, out, "bucket")
	assert.Contains(t, out, "AKIA")
	assert.NotContains(t, out, "s3cr3t")
}

func TestFmeConfig_Redacted(t *testing.T) {
	cfg := FmeConfig{}
	cfg.Protocols.S3.TransferProfiles = map[string]TransferProfile{
		"keys": {Name: "keys", StaticCredentials: &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "s3cr3t"}},
		"aws":  {Name: "aws", Profile: "default"},
	}

	redacted := cfg.Redacted()
	assert.Equal(t, RedactedSecret, redacted.Protocols.S3.TransferProfiles["keys"].StaticCredentials.SecretAccessKey)
	assert.Nil(t, redacted.Protocols.S3.TransferProfiles["aws"].StaticCredentials)
	assert.Equal(t, "default", redacted.Protocols.S3.TransferProfiles["aws"].Profile)
	// Source config must not be modified.
	assert.Equal(t, "s3cr3t", cfg.Protocols.S3.TransferProfiles["keys"].StaticCredentials.SecretAccessKey)
}

func TestTransferProfileFromProtobuf_NilSubMessages(t *testing.T) {
	assert.Equal(t, TransferProfile{}, TransferProfileFromProtobuf(nil))

	tp := TransferProfileFromProtobuf(&fmev1.TransferProfile{Name: "bare", Bucket: "b"})
	assert.Equal(t, "bare", tp.Name)
	assert.Equal(t, PathsSettings{}, tp.Paths)
	assert.Equal(t, ChecksumSettings{}, tp.Checksums)
	assert.Nil(t, tp.OIDCConfig)
	assert.Nil(t, tp.StaticCredentials)
	assert.Equal(t, EndpointTLSSettings{}, tp.TLS)
	assert.Equal(t, S3ProviderUnspecified, tp.Provider)
}

func TestTransferProfile_ProtobufRoundTrip(t *testing.T) {
	original := TransferProfile{
		Name:                 "minio-local",
		Bucket:               "media",
		Region:               "us-east-1",
		Endpoint:             "http://127.0.0.1:9000",
		Filter:               ".*",
		AutoTuning:           true,
		Checksums:            ChecksumSettings{Enabled: true, Algorithm: "xxh3"},
		ChunkSize:            25,
		Threads:              4,
		MaxAge:               "7d",
		FileOrder:            []string{".mov"},
		EnableMetadataFilter: true,
		StorageClass:         "standard",
		Paths:                PathsSettings{Local: "/data", Remote: "in/"},
		AuthMethod:           AuthMethodStaticKeys,
		Provider:             S3ProviderMinIO,
		PathStyle:            true,
		StaticCredentials:    &StaticCredentials{AccessKeyID: "AKIA", SecretAccessKey: "s3cr3t", SessionToken: "tok"},
		TLS:                  EndpointTLSSettings{CABundle: "/etc/ca.pem", SkipVerify: true},
	}

	pb := TransferProfileToProtobuf(original)
	require.NotNil(t, pb.StaticCredentials)
	assert.Equal(t, fmev1.S3Provider_S3_PROVIDER_MINIO, pb.Provider)
	assert.Equal(t, fmev1.AuthMethod_AUTH_METHOD_STATIC_KEYS, pb.AuthMethod)
	assert.True(t, pb.PathStyle)
	assert.Equal(t, "/etc/ca.pem", pb.Tls.CaBundle)

	roundTripped := TransferProfileFromProtobuf(pb)
	assert.Equal(t, original, roundTripped)

	// Map-level helpers delegate to the single-profile converters.
	fromMap := TransferProfilesFromProtobuf(TransferProfilesToProtobuf(map[string]TransferProfile{"minio-local": original}))
	assert.Equal(t, original, fromMap["minio-local"])

	// An AWS profile without static credentials must not grow a credentials block.
	aws := TransferProfile{Name: "aws", Profile: "default", Region: "us-east-2"}
	awsPb := TransferProfileToProtobuf(aws)
	assert.Nil(t, awsPb.StaticCredentials)
	assert.Equal(t, fmev1.S3Provider_S3_PROVIDER_UNSPECIFIED, awsPb.Provider, "legacy profiles stay unspecified on the wire")
	awsRoundTripped := TransferProfileFromProtobuf(awsPb)
	assert.Equal(t, aws, awsRoundTripped)
	assert.False(t, strings.Contains(awsRoundTripped.String(), RedactedSecret))
}
