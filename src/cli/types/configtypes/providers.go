package configtypes

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/awslabs/filemoverexpress/types/pbtypes/fme/v1"
)

const (
	// S3ProviderUnspecified is the zero value and is treated identically to S3ProviderAWS so that
	// configuration files written before the provider field existed keep working.
	S3ProviderUnspecified  S3Provider = ""
	S3ProviderAWS          S3Provider = "aws"
	S3ProviderMinIO        S3Provider = "minio"
	S3ProviderCloudflare   S3Provider = "cloudflare-r2"
	S3ProviderBackblaze    S3Provider = "backblaze-b2"
	S3ProviderWasabi       S3Provider = "wasabi"
	S3ProviderDigitalOcean S3Provider = "digitalocean-spaces"
	S3ProviderHetzner      S3Provider = "hetzner"
	// S3ProviderOther covers any S3-compatible endpoint that has no dedicated preset.
	S3ProviderOther S3Provider = "other"

	// RedactedSecret replaces secret values when a configuration is printed or exported.
	RedactedSecret = "***REDACTED***"

	connectionKeySeparator = "|"
	// secretFingerprintBytes is how much of the SHA-256 of the secret goes into a connection key.
	secretFingerprintBytes = 8
)

var (
	// providerSpecs is the single source of truth for per-provider behaviour. The GUI mirrors the
	// user-facing parts of this table (endpoint template, region default, path style) as presets.
	providerSpecs = map[S3Provider]ProviderSpec{
		S3ProviderAWS: {
			IsAWS:                true,
			DefaultRegion:        "us-east-2",
			SupportsVersioning:   true,
			SupportsStorageClass: true,
			SupportsOIDC:         true,
		},
		S3ProviderMinIO: {
			DefaultRegion:        "us-east-1",
			EndpointTemplate:     "https://minio.example.com:9000",
			DefaultPathStyle:     true,
			SupportsVersioning:   true,
			SupportsStorageClass: true,
		},
		S3ProviderCloudflare: {
			DefaultRegion:        "auto",
			EndpointTemplate:     "https://{account_id}.r2.cloudflarestorage.com",
			SupportsVersioning:   false,
			SupportsStorageClass: true,
		},
		S3ProviderBackblaze: {
			DefaultRegion:        "us-west-004",
			EndpointTemplate:     "https://s3.{region}.backblazeb2.com",
			SupportsVersioning:   true,
			SupportsStorageClass: false,
		},
		S3ProviderWasabi: {
			DefaultRegion:        "us-east-1",
			EndpointTemplate:     "https://s3.{region}.wasabisys.com",
			SupportsVersioning:   true,
			SupportsStorageClass: false,
		},
		S3ProviderDigitalOcean: {
			DefaultRegion:        "us-east-1",
			EndpointTemplate:     "https://{region}.digitaloceanspaces.com",
			SupportsVersioning:   false,
			SupportsStorageClass: false,
		},
		// Hetzner Object Storage: the region must be the bucket's location code (fsn1, nbg1, hel1) and
		// the endpoint carries the same code. Virtual-hosted addressing is what Hetzner documents.
		S3ProviderHetzner: {
			DefaultRegion:        "fsn1",
			EndpointTemplate:     "https://{region}.your-objectstorage.com",
			SupportsVersioning:   true,
			SupportsStorageClass: false,
		},
		S3ProviderOther: {
			DefaultRegion:        "us-east-1",
			DefaultPathStyle:     true,
			SupportsVersioning:   true,
			SupportsStorageClass: true,
		},
	}

	providerToProtobuf = map[S3Provider]fmev1.S3Provider{
		S3ProviderAWS:          fmev1.S3Provider_S3_PROVIDER_AWS,
		S3ProviderMinIO:        fmev1.S3Provider_S3_PROVIDER_MINIO,
		S3ProviderCloudflare:   fmev1.S3Provider_S3_PROVIDER_CLOUDFLARE_R2,
		S3ProviderBackblaze:    fmev1.S3Provider_S3_PROVIDER_BACKBLAZE_B2,
		S3ProviderWasabi:       fmev1.S3Provider_S3_PROVIDER_WASABI,
		S3ProviderDigitalOcean: fmev1.S3Provider_S3_PROVIDER_DIGITALOCEAN_SPACES,
		S3ProviderHetzner:      fmev1.S3Provider_S3_PROVIDER_HETZNER,
		S3ProviderOther:        fmev1.S3Provider_S3_PROVIDER_OTHER,
	}

	ErrEndpointRequired      = errors.New(strEndpointRequired)
	ErrEndpointInvalid       = errors.New(strEndpointInvalid)
	ErrOIDCUnsupported       = errors.New(strOIDCUnsupported)
	ErrStaticKeysIncomplete  = errors.New(strStaticKeysIncomplete)
	ErrSecretMustBeReentered = errors.New(strSecretMustBeReentered)
)

type (
	// S3Provider identifies which S3 implementation a TransferProfile talks to. Stored as a short
	// lower-case string in the configuration file (e.g. "minio") and mapped to the protobuf enum on the wire.
	S3Provider string

	// ProviderSpec describes how FME must talk to a given provider.
	ProviderSpec struct {
		// IsAWS enables AWS-only behaviour: STS session validation, OIDC auth, AWS storage classes.
		IsAWS bool
		// DefaultRegion is used when a profile has no region configured.
		DefaultRegion string
		// EndpointTemplate is a documentation/GUI hint; {account_id} and {region} are placeholders.
		EndpointTemplate string
		// DefaultPathStyle is the recommended addressing style for new profiles.
		DefaultPathStyle bool
		// SupportsVersioning is false for providers that do not implement ListObjectVersions.
		SupportsVersioning bool
		// SupportsStorageClass is false for providers that reject the x-amz-storage-class header.
		SupportsStorageClass bool
		// SupportsOIDC is true only where OIDC → STS AssumeRoleWithWebIdentity is available.
		SupportsOIDC bool
	}
)

// canonical lower-cases and trims the value so that hand-edited configuration files ("MinIO ") match.
func (p S3Provider) canonical() S3Provider {
	return S3Provider(strings.ToLower(strings.TrimSpace(string(p))))
}

// Normalized maps the empty value to AWS and any unknown value to Other.
func (p S3Provider) Normalized() S3Provider {
	canonical := p.canonical()
	if canonical == S3ProviderUnspecified {
		return S3ProviderAWS
	}
	if _, known := providerSpecs[canonical]; known {
		return canonical
	}
	return S3ProviderOther
}

// IsKnown reports whether the value is empty or one of the supported provider identifiers. Unknown values
// still work: they are treated as S3ProviderOther, and configuration loading warns about them.
func (p S3Provider) IsKnown() bool {
	canonical := p.canonical()
	if canonical == S3ProviderUnspecified {
		return true
	}
	_, known := providerSpecs[canonical]
	return known
}

// Spec returns the behaviour table entry for the (normalized) provider.
func (p S3Provider) Spec() ProviderSpec {
	return providerSpecs[p.Normalized()]
}

// IsAWS reports whether the profile targets Amazon S3 (including the legacy empty value).
func (p S3Provider) IsAWS() bool {
	return p.Spec().IsAWS
}

// ToProtobuf converts the provider to its wire representation. The legacy empty value stays UNSPECIFIED so
// that a configuration round-tripped through the GUI does not gain a provider key unless the user picks one.
// Unknown values map to OTHER.
func (p S3Provider) ToProtobuf() fmev1.S3Provider {
	if p.canonical() == S3ProviderUnspecified {
		return fmev1.S3Provider_S3_PROVIDER_UNSPECIFIED
	}
	if pb, ok := providerToProtobuf[p.Normalized()]; ok {
		return pb
	}
	return fmev1.S3Provider_S3_PROVIDER_OTHER
}

// S3ProviderFromProtobuf converts the wire enum to the configuration representation. UNSPECIFIED
// becomes the empty value so that round-tripping a legacy profile does not add a provider key.
func S3ProviderFromProtobuf(pb fmev1.S3Provider) S3Provider {
	if pb == fmev1.S3Provider_S3_PROVIDER_UNSPECIFIED {
		return S3ProviderUnspecified
	}
	for provider, candidate := range providerToProtobuf {
		if candidate == pb {
			return provider
		}
	}
	return S3ProviderOther
}

// ValidateProfileForProvider checks the provider-dependent invariants of a transfer profile. It is
// called when an S3 session is created rather than at configuration load, so that an invalid profile
// surfaces as a clear error at connect time instead of silently rewriting the user's file.
func ValidateProfileForProvider(tp TransferProfile) error {
	spec := tp.Provider.Spec()
	if !spec.IsAWS {
		if err := validateEndpointURL(tp.Endpoint); err != nil {
			return err
		}
	}
	if tp.AuthMethod == AuthMethodOIDC && !spec.SupportsOIDC {
		return ErrOIDCUnsupported
	}
	if tp.AuthMethod == AuthMethodStaticKeys && !tp.StaticCredentials.isComplete() {
		return ErrStaticKeysIncomplete
	}
	return nil
}

func validateEndpointURL(endpoint string) error {
	if strings.TrimSpace(endpoint) == "" {
		return ErrEndpointRequired
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ErrEndpointInvalid
	}
	return nil
}

// ConnectionKey identifies the S3 connection a profile resolves to. Profiles with the same key can
// share a cached client. Secrets only contribute a short hash, so that rotating a secret or session token
// produces a new key without the secret itself appearing in the key.
func (tp TransferProfile) ConnectionKey() string {
	accessKeyID, secretFingerprint := "", ""
	if tp.StaticCredentials != nil {
		accessKeyID = tp.StaticCredentials.AccessKeyID
		secretFingerprint = tp.StaticCredentials.fingerprint()
	}
	return strings.Join([]string{
		tp.Name,
		string(tp.Provider.Normalized()),
		tp.Bucket,
		tp.Region,
		tp.Endpoint,
		strconv.FormatBool(tp.PathStyle),
		strconv.Itoa(int(tp.AuthMethod)),
		tp.Profile,
		accessKeyID,
		secretFingerprint,
		tp.TLS.CABundle,
		strconv.FormatBool(tp.TLS.SkipVerify),
	}, connectionKeySeparator)
}

func (c *StaticCredentials) fingerprint() string {
	sum := sha256.Sum256([]byte(c.SecretAccessKey + connectionKeySeparator + c.SessionToken))
	return hex.EncodeToString(sum[:secretFingerprintBytes])
}

func (c *StaticCredentials) isComplete() bool {
	return c != nil && strings.TrimSpace(c.AccessKeyID) != "" && strings.TrimSpace(c.SecretAccessKey) != ""
}

// String renders the credentials with the secret parts redacted. It is used by fmt (and therefore by
// TransferProfile.String) so that secrets never reach logs, job ids or support files.
func (c *StaticCredentials) String() string {
	if c == nil {
		return "<nil>"
	}
	return "{AccessKeyID:" + c.AccessKeyID + " SecretAccessKey:" + RedactedSecret + " SessionToken:" + RedactedSecret + "}"
}

// GoString mirrors String for the %#v verb.
func (c *StaticCredentials) GoString() string {
	return c.String()
}

// Redacted returns a copy with the secret access key and session token replaced by a placeholder.
func (c *StaticCredentials) Redacted() *StaticCredentials {
	if c == nil {
		return nil
	}
	out := *c
	if out.SecretAccessKey != "" {
		out.SecretAccessKey = RedactedSecret
	}
	if out.SessionToken != "" {
		out.SessionToken = RedactedSecret
	}
	return &out
}

// RestoreRedactedSecrets replaces RedactedSecret placeholders sent back by a client with the secrets stored
// for the profile of the same name. The daemon never sends stored secrets to clients, so a client that did
// not change a secret returns the placeholder. A placeholder is only honoured while the profile name and
// access key ID are unchanged; otherwise the user has to enter the secret again.
func (tp *TransferProfile) RestoreRedactedSecrets(stored map[string]TransferProfile) error {
	creds := tp.StaticCredentials
	if creds == nil || (creds.SecretAccessKey != RedactedSecret && creds.SessionToken != RedactedSecret) {
		return nil
	}
	previous, found := stored[tp.Name]
	if !found || previous.StaticCredentials == nil || previous.StaticCredentials.AccessKeyID != creds.AccessKeyID {
		return fmt.Errorf("%w: %s", ErrSecretMustBeReentered, tp.Name)
	}
	restored := *creds
	if restored.SecretAccessKey == RedactedSecret {
		restored.SecretAccessKey = previous.StaticCredentials.SecretAccessKey
	}
	if restored.SessionToken == RedactedSecret {
		restored.SessionToken = previous.StaticCredentials.SessionToken
	}
	tp.StaticCredentials = &restored
	return nil
}
