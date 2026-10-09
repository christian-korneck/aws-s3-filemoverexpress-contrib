package transfer_api

import (
	"errors"
	"net/http"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
)

var (
	// unsupportedOperationCodes are the error codes S3-compatible services return for operations they do
	// not implement (for example ListObjectVersions on a store without versioning).
	unsupportedOperationCodes = map[string]bool{
		"NotImplemented":   true,
		"MethodNotAllowed": true,
		"NotSupported":     true,
		"XNotImplemented":  true,
	}

	// batchDeleteRejectedCodes are returned by S3-compatible services that require a Content-MD5 header on
	// DeleteObjects. The SDK only sends a CRC32 checksum, but single-object deletes need neither.
	batchDeleteRejectedCodes = map[string]bool{
		"MissingContentMD5": true,
		"InvalidDigest":     true,
	}
)

// isBatchDeleteRejected reports whether a DeleteObjects error means the objects should be deleted one at a time.
func isBatchDeleteRejected(err error) bool {
	if isUnsupportedOperation(err) {
		return true
	}
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && batchDeleteRejectedCodes[apiErr.ErrorCode()]
}

// isUnsupportedOperation reports whether an S3 error means the endpoint does not implement the operation,
// so that the caller can fall back to a simpler API call.
func isUnsupportedOperation(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && unsupportedOperationCodes[apiErr.ErrorCode()] {
		return true
	}
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		code := respErr.HTTPStatusCode()
		return code == http.StatusNotImplemented || code == http.StatusMethodNotAllowed
	}
	return false
}
