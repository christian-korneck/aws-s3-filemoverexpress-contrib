package service

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awslabs/filemoverexpress/constants"
	transferapi "github.com/awslabs/filemoverexpress/core/transfer-api"
	"github.com/awslabs/filemoverexpress/types/configtypes"
	"github.com/awslabs/filemoverexpress/types/pbtypes/fme/v1"
)

const emptyListBucketResult = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>media</Name><Prefix>/</Prefix><KeyCount>0</KeyCount><MaxKeys>1</MaxKeys><IsTruncated>false</IsTruncated>
</ListBucketResult>`

// startFakeS3 serves an empty ListObjectsV2 result over TLS with a self-signed certificate and returns the
// server plus the path of a PEM file containing that certificate.
func startFakeS3(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("list-type") != "2" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(emptyListBucketResult))
	}))
	t.Cleanup(server.Close)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, os.WriteFile(caPath, pemBytes, 0o600))
	return server, caPath
}

func staticKeysProfile(endpoint string, tls configtypes.EndpointTLSSettings) configtypes.TransferProfile {
	return configtypes.TransferProfile{
		Name:       "fake-s3",
		Bucket:     "media",
		Region:     "us-east-1",
		Provider:   configtypes.S3ProviderOther,
		Endpoint:   endpoint,
		PathStyle:  true,
		AuthMethod: configtypes.AuthMethodStaticKeys,
		StaticCredentials: &configtypes.StaticCredentials{
			AccessKeyID: "AKIATEST", SecretAccessKey: "secret",
		},
		TLS: tls,
	}
}

func TestValidateTransferProfile_Probe(t *testing.T) {
	server, caPath := startFakeS3(t)

	t.Run("succeeds with the server CA bundle", func(t *testing.T) {
		msg, ok := validateTransferProfile(context.Background(),
			staticKeysProfile(server.URL, configtypes.EndpointTLSSettings{CABundle: caPath}))
		assert.True(t, ok, msg)
		assert.Contains(t, msg, "media")
	})

	t.Run("fails without trusting the certificate", func(t *testing.T) {
		msg, ok := validateTransferProfile(context.Background(),
			staticKeysProfile(server.URL, configtypes.EndpointTLSSettings{}))
		assert.False(t, ok)
		assert.Contains(t, msg, "certificate")
		assert.NotContains(t, msg, "secret")
	})

	t.Run("succeeds when verification is skipped", func(t *testing.T) {
		msg, ok := validateTransferProfile(context.Background(),
			staticKeysProfile(server.URL, configtypes.EndpointTLSSettings{SkipVerify: true}))
		assert.True(t, ok, msg)
	})

	t.Run("reports provider validation errors", func(t *testing.T) {
		profile := staticKeysProfile(server.URL, configtypes.EndpointTLSSettings{CABundle: caPath})
		profile.Provider = configtypes.S3ProviderMinIO
		profile.AuthMethod = configtypes.AuthMethodOIDC
		msg, ok := validateTransferProfile(context.Background(), profile)
		assert.False(t, ok)
		assert.Contains(t, msg, "OIDC")
	})

	t.Run("reports an unreachable endpoint", func(t *testing.T) {
		profile := staticKeysProfile("http://127.0.0.1:1", configtypes.EndpointTLSSettings{})
		msg, ok := validateTransferProfile(context.Background(), profile)
		assert.False(t, ok)
		assert.Contains(t, msg, "media")
	})
}

func TestProfileToValidate(t *testing.T) {
	stored := map[string]configtypes.TransferProfile{
		"r2": {Name: "r2", StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "AKIATEST", SecretAccessKey: "stored-secret"}},
	}

	t.Run("applies name and provider default region", func(t *testing.T) {
		tp, err := profileToValidate(&fmev1.TransferProfile{Provider: fmev1.S3Provider_S3_PROVIDER_CLOUDFLARE_R2}, stored)
		require.NoError(t, err)
		assert.Equal(t, unsavedProfileName, tp.Name)
		assert.Equal(t, "auto", tp.Region)
	})
	t.Run("keeps an explicit region", func(t *testing.T) {
		tp, err := profileToValidate(&fmev1.TransferProfile{Name: "x", Region: "eu-west-1"}, stored)
		require.NoError(t, err)
		assert.Equal(t, "eu-west-1", tp.Region)
	})
	t.Run("restores a stored secret", func(t *testing.T) {
		tp, err := profileToValidate(&fmev1.TransferProfile{
			Name:              "r2",
			AuthMethod:        fmev1.AuthMethod_AUTH_METHOD_STATIC_KEYS,
			StaticCredentials: &fmev1.StaticCredentials{AccessKeyId: "AKIATEST", SecretAccessKey: configtypes.RedactedSecret},
		}, stored)
		require.NoError(t, err)
		assert.Equal(t, "stored-secret", tp.StaticCredentials.SecretAccessKey)
	})
	t.Run("rejects a placeholder for an unsaved profile", func(t *testing.T) {
		_, err := profileToValidate(&fmev1.TransferProfile{
			AuthMethod:        fmev1.AuthMethod_AUTH_METHOD_STATIC_KEYS,
			StaticCredentials: &fmev1.StaticCredentials{AccessKeyId: "AKIATEST", SecretAccessKey: configtypes.RedactedSecret},
		}, stored)
		assert.ErrorIs(t, err, configtypes.ErrSecretMustBeReentered)
	})
}

func TestNewS3ManagerContext_TimesOut(t *testing.T) {
	release := make(chan struct{})
	original := newS3ManagerFunc
	newS3ManagerFunc = func(configtypes.TransferProfile) (*transferapi.S3Manager, error) {
		<-release
		return nil, nil
	}
	t.Cleanup(func() {
		close(release)
		newS3ManagerFunc = original
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := newS3ManagerContext(ctx, configtypes.TransferProfile{})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRestoreRedactedSecrets_SetConfiguration(t *testing.T) {
	stored := map[string]configtypes.TransferProfile{
		"minio": {Name: "minio", StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "AKIATEST", SecretAccessKey: "stored-secret"}},
	}
	newConfig := configtypes.FmeConfig{}
	newConfig.Protocols.S3.TransferProfiles = map[string]configtypes.TransferProfile{
		"minio": {Name: "minio", StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "AKIATEST", SecretAccessKey: configtypes.RedactedSecret}},
		"aws":   {Name: "aws"},
	}
	require.NoError(t, restoreRedactedSecrets(&newConfig, stored))
	assert.Equal(t, "stored-secret", newConfig.Protocols.S3.TransferProfiles["minio"].StaticCredentials.SecretAccessKey)
	assert.Nil(t, newConfig.Protocols.S3.TransferProfiles["aws"].StaticCredentials)

	newConfig.Protocols.S3.TransferProfiles["minio"] = configtypes.TransferProfile{
		Name: "minio", StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "CHANGED", SecretAccessKey: configtypes.RedactedSecret},
	}
	assert.ErrorIs(t, restoreRedactedSecrets(&newConfig, stored), configtypes.ErrSecretMustBeReentered)
}

func TestGetConfigurationPayload_HidesSecrets(t *testing.T) {
	cfg := configtypes.FmeConfig{}
	cfg.Protocols.S3.TransferProfiles = map[string]configtypes.TransferProfile{
		"minio": {
			Name: "minio", AuthMethod: configtypes.AuthMethodStaticKeys,
			StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "AKIATEST", SecretAccessKey: "stored-secret", SessionToken: "tok"},
		},
	}
	creds := cfg.Redacted().ToGRPCProtobuf().GetProtocols().GetS3().GetTransferProfiles()["minio"].GetStaticCredentials()
	assert.Equal(t, "AKIATEST", creds.GetAccessKeyId())
	assert.Equal(t, configtypes.RedactedSecret, creds.GetSecretAccessKey())
	assert.Equal(t, configtypes.RedactedSecret, creds.GetSessionToken())
}

func TestValidateTransferProfile_Handler(t *testing.T) {
	srv := &FileMoverServer{}
	// A request built in-process has no peer address, so it is treated as a remote client.
	_, err := srv.ValidateTransferProfile(context.Background(),
		connect.NewRequest(&fmev1.ValidateTransferProfileRequest{}))
	require.Error(t, err)
	if constants.DefaultAllowUIConfiguration {
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	} else {
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	}
}
