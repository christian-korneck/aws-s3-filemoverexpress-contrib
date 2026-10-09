package transfer_api

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"sync"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"

	"github.com/awslabs/filemoverexpress/core/auth"
	"github.com/awslabs/filemoverexpress/events"
	"github.com/awslabs/filemoverexpress/logger"
	"github.com/awslabs/filemoverexpress/types/configtypes"
)

var (
	// skipVerifyWarned remembers which profiles already produced the skip-verify warning, so the log and the
	// GUI event log are not flooded every time a client is (re)built for the same profile.
	skipVerifyWarned sync.Map

	// endpointHTTPClients shares one HTTP client (and its connection pool) per TLS setting, because S3
	// managers are built per request for browsing, renaming and deleting.
	endpointHTTPClients     = make(map[configtypes.EndpointTLSSettings]*awshttp.BuildableClient)
	endpointHTTPClientsLock sync.Mutex
)

// buildEndpointHTTPClient returns an HTTP client honouring the profile's TLS settings, or nil when the SDK
// default client should be used. The client is attached per S3 client so the shared aws.Config cache is
// unaffected by profile-specific trust settings.
func buildEndpointHTTPClient(settings configtypes.EndpointTLSSettings, profileName string) (*awshttp.BuildableClient, error) {
	if settings.CABundle == "" && !settings.SkipVerify {
		return nil, nil
	}
	if settings.SkipVerify {
		if _, already := skipVerifyWarned.LoadOrStore(profileName, true); !already {
			logger.Warn(strSkipTLSVerifyWarning, profileName)
			events.Events.Warn(strSkipTLSVerifyWarning, profileName)
		}
	}

	endpointHTTPClientsLock.Lock()
	defer endpointHTTPClientsLock.Unlock()
	if client, ok := endpointHTTPClients[settings]; ok {
		return client, nil
	}

	var pool *x509.CertPool
	if settings.CABundle != "" {
		var err error
		if pool, err = auth.LoadCABundlePool(settings.CABundle); err != nil {
			return nil, err
		}
	}
	client := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
		// Start from the SDK's TLS settings (minimum version, FIPS curve preferences) and only change trust.
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		if tr.TLSClientConfig != nil {
			tlsConfig = tr.TLSClientConfig.Clone()
		}
		if pool != nil {
			tlsConfig.RootCAs = pool
		}
		if settings.SkipVerify {
			tlsConfig.InsecureSkipVerify = true //nolint:gosec // explicit, user-chosen opt-out for self-signed lab endpoints
		}
		tr.TLSClientConfig = tlsConfig
	})
	endpointHTTPClients[settings] = client
	return client, nil
}

// ResetEndpointHTTPClients drops the shared endpoint HTTP clients, so that a configuration reload re-reads
// CA bundle files.
func ResetEndpointHTTPClients() {
	endpointHTTPClientsLock.Lock()
	endpointHTTPClients = make(map[configtypes.EndpointTLSSettings]*awshttp.BuildableClient)
	endpointHTTPClientsLock.Unlock()
}
