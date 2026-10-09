package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/awslabs/filemoverexpress/types/configtypes"
)

func TestLoadConfig_S3CompatibleProfile(t *testing.T) {
	resetConfigState()
	defer resetConfigState()

	cfgPath := filepath.Join(t.TempDir(), "configuration.yaml")
	fixture := `protocols:
  s3:
    transferProfiles:
      minio-local:
        name: "minio-local"
        bucket: "media"
        provider: "minio"
        endpoint: "http://127.0.0.1:9000"
        pathStyle: true
        authMethod: 3
        staticCredentials:
          accessKeyId: "minioadmin"
          secretAccessKey: "minioadmin123"
          sessionToken: "tok"
        tls:
          caBundle: "/etc/fme/ca.pem"
          skipVerify: true
        threads: 4
        chunkSize: 25
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(fixture), 0600))
	setupFileProvider(cfgPath)

	cfg := loadConfig()
	profile, ok := cfg.Protocols.S3.TransferProfiles["minio-local"]
	require.True(t, ok)

	assert.Equal(t, configtypes.S3ProviderMinIO, profile.Provider)
	assert.Equal(t, "http://127.0.0.1:9000", profile.Endpoint)
	assert.True(t, profile.PathStyle)
	assert.Equal(t, configtypes.AuthMethodStaticKeys, profile.AuthMethod)
	require.NotNil(t, profile.StaticCredentials)
	assert.Equal(t, "minioadmin", profile.StaticCredentials.AccessKeyID)
	assert.Equal(t, "minioadmin123", profile.StaticCredentials.SecretAccessKey)
	assert.Equal(t, "tok", profile.StaticCredentials.SessionToken)
	assert.Equal(t, "/etc/fme/ca.pem", profile.TLS.CABundle)
	assert.True(t, profile.TLS.SkipVerify)
	assert.NoError(t, configtypes.ValidateProfileForProvider(profile))
}

func TestValidateTransferProfiles_RegionDefaultsPerProvider(t *testing.T) {
	cfg := &configtypes.FmeConfig{}
	cfg.Protocols.S3.TransferProfiles = map[string]configtypes.TransferProfile{
		"aws":      {Name: "aws"},
		"r2":       {Name: "r2", Provider: configtypes.S3ProviderCloudflare, Endpoint: "https://acct.r2.cloudflarestorage.com"},
		"b2":       {Name: "b2", Provider: configtypes.S3ProviderBackblaze, Endpoint: "https://s3.eu-central-003.backblazeb2.com", Region: "eu-central-003"},
		"unknown":  {Name: "unknown", Provider: "bogus", Endpoint: "https://s3.local"},
		"explicit": {Name: "explicit", Provider: configtypes.S3ProviderMinIO, Endpoint: "https://m", Region: "eu-west-1"},
	}

	assert.True(t, validateTransferProfiles(cfg))

	profiles := cfg.Protocols.S3.TransferProfiles
	assert.Equal(t, "us-east-2", profiles["aws"].Region, "legacy AWS default must not change")
	assert.Equal(t, "auto", profiles["r2"].Region)
	assert.Equal(t, "eu-central-003", profiles["b2"].Region, "an explicit region is kept")
	assert.Equal(t, "us-east-1", profiles["unknown"].Region, "unknown providers use the generic default")
	assert.Equal(t, configtypes.S3Provider("bogus"), profiles["unknown"].Provider, "unknown provider values are warned about, not rewritten")
	assert.Equal(t, "eu-west-1", profiles["explicit"].Region)
}

func TestTransferProfileYAML_LegacyProfileGainsNoProviderKeys(t *testing.T) {
	legacy := configtypes.TransferProfile{Name: "aws", Bucket: "b", Region: "us-east-2", Profile: "default"}
	out, err := yamlv3.Marshal(legacy)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "provider:")
	assert.NotContains(t, string(out), "staticCredentials:")
	assert.NotContains(t, string(out), "pathStyle:")
	assert.NotContains(t, string(out), "skipVerify:")
	assert.NotContains(t, string(out), "caBundle:")

	minio := configtypes.TransferProfile{
		Name:              "minio",
		Provider:          configtypes.S3ProviderMinIO,
		StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "a", SecretAccessKey: "s"},
	}
	out, err = yamlv3.Marshal(minio)
	require.NoError(t, err)
	assert.Contains(t, string(out), "provider: minio")
	assert.Contains(t, string(out), "accessKeyId: a")
	assert.NotContains(t, string(out), "sessionToken:", "empty optional token is omitted")
}

func TestSaveConfig_RoundTripsProviderSettings(t *testing.T) {
	resetConfigState()
	defer resetConfigState()

	cfgPath := filepath.Join(t.TempDir(), "configuration.yaml")
	cfg := buildDefaultConfig()
	cfg.Protocols.S3.TransferProfiles = map[string]configtypes.TransferProfile{
		"minio": {
			Name:              "minio",
			Bucket:            "media",
			Provider:          configtypes.S3ProviderMinIO,
			Endpoint:          "https://minio.local:9000",
			PathStyle:         true,
			AuthMethod:        configtypes.AuthMethodStaticKeys,
			StaticCredentials: &configtypes.StaticCredentials{AccessKeyID: "AKIATEST", SecretAccessKey: "s3cr3t", SessionToken: "tok"},
			TLS:               configtypes.EndpointTLSSettings{CABundle: "/etc/fme/ca.pem", SkipVerify: true},
		},
	}
	require.NoError(t, marshalConfigToFile(cfg, cfgPath))
	setupFileProvider(cfgPath)

	profile := loadConfig().Protocols.S3.TransferProfiles["minio"]
	assert.Equal(t, configtypes.S3ProviderMinIO, profile.Provider)
	assert.Equal(t, "https://minio.local:9000", profile.Endpoint)
	assert.True(t, profile.PathStyle)
	assert.Equal(t, configtypes.AuthMethodStaticKeys, profile.AuthMethod)
	require.NotNil(t, profile.StaticCredentials)
	assert.Equal(t, configtypes.StaticCredentials{AccessKeyID: "AKIATEST", SecretAccessKey: "s3cr3t", SessionToken: "tok"}, *profile.StaticCredentials)
	assert.Equal(t, configtypes.EndpointTLSSettings{CABundle: "/etc/fme/ca.pem", SkipVerify: true}, profile.TLS)
}

func TestMarshalConfigToFile_RestrictsExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not enforced on Windows")
	}
	cfgPath := filepath.Join(t.TempDir(), "configuration.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{}\n"), 0644)) //nolint:gosec // simulates a file created by an older version
	require.NoError(t, os.Chmod(cfgPath, 0644))

	require.NoError(t, marshalConfigToFile(buildDefaultConfig(), cfgPath))

	info, err := os.Stat(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, configFileMode, info.Mode().Perm())
}
