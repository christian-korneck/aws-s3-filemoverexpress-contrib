package supportfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awslabs/filemoverexpress/types/configtypes"
)

func TestMarshalStructs_RedactedConfigHidesStaticSecrets(t *testing.T) {
	cfg := configtypes.FmeConfig{}
	cfg.Protocols.S3.TransferProfiles = map[string]configtypes.TransferProfile{
		"minio": {
			Name:       "minio",
			Provider:   configtypes.S3ProviderMinIO,
			Endpoint:   "http://127.0.0.1:9000",
			AuthMethod: configtypes.AuthMethodStaticKeys,
			StaticCredentials: &configtypes.StaticCredentials{
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "sup3rs3cret",
				SessionToken:    "t0ken",
			},
		},
	}

	// The unredacted config would leak the secret; the support file must use Redacted().
	raw, err := marshalStructs(cfg)
	require.NoError(t, err)
	assert.Contains(t, raw, "sup3rs3cret")

	redacted, err := marshalStructs(cfg.Redacted())
	require.NoError(t, err)
	assert.NotContains(t, redacted, "sup3rs3cret")
	assert.NotContains(t, redacted, "t0ken")
	assert.Contains(t, redacted, "minioadmin")
	assert.Contains(t, redacted, configtypes.RedactedSecret)
	assert.Contains(t, redacted, "provider: minio")
}
