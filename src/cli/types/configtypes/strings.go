package configtypes

//revive:disable:line-length-limit
const (
	strErrorWritingConfig    = "Error writing configuration file: %s\n"
	strNoSuchTransferProfile = "No such transfer profile configuration: "
	strEndpointRequired      = "an endpoint URL is required for this S3 provider"
	strEndpointInvalid       = "the endpoint must be a full URL starting with http:// or https://"
	strOIDCUnsupported       = "OIDC / SSO authentication is only available for Amazon S3"
	strStaticKeysIncomplete  = "static credentials require both an access key ID and a secret access key"
	strSecretMustBeReentered = "the secret access key must be entered again because the Remote Configuration name or access key ID changed"
)
