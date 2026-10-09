package transfer_api

//revive:disable:line-length-limit
const (
	strSkipTLSVerifyWarning           = "TLS certificate verification is disabled for remote configuration '%s'; connections to its endpoint are not authenticated"
	strVersioningUnsupportedFallback  = "Bucket %s does not support object versioning (%s); deleting current objects by key instead"
	strBatchDeleteUnsupportedFallback = "Bucket %s rejected the batch delete request (%s); deleting objects one at a time"
	strDeleteObjectFailed             = "Error deleting %s from bucket %s: %s"
	strDeleteObjectSucceeded          = "Successfully deleted %s from bucket %s"
	strFoundUnversionedObjects        = "Found %d current object(s) for %s (object versioning not available)"
	strRenameStorageClassInvalid      = "error renaming object %s with storage class %s for rename: %w"
)
