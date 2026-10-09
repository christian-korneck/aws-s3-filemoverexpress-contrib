package transfer_api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awslabs/filemoverexpress/types/configtypes"
)

// writeSelfSignedPEM writes a throwaway self-signed certificate and returns its path.
func writeSelfSignedPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "fme-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestBuildEndpointHTTPClient(t *testing.T) {
	t.Run("defaults return nil client", func(t *testing.T) {
		client, err := buildEndpointHTTPClient(configtypes.EndpointTLSSettings{}, "p")
		if err != nil || client != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", client, err)
		}
	})

	t.Run("missing CA bundle is an error", func(t *testing.T) {
		_, err := buildEndpointHTTPClient(configtypes.EndpointTLSSettings{CABundle: "/nope.pem"}, "p")
		if err == nil {
			t.Fatal("expected error for missing CA bundle")
		}
	})

	t.Run("CA bundle is loaded into the root pool", func(t *testing.T) {
		client, err := buildEndpointHTTPClient(configtypes.EndpointTLSSettings{CABundle: writeSelfSignedPEM(t)}, "p")
		if err != nil {
			t.Fatalf("buildEndpointHTTPClient() error = %v", err)
		}
		tlsConfig := client.GetTransport().TLSClientConfig
		if tlsConfig == nil || tlsConfig.RootCAs == nil {
			t.Fatal("expected a TLS config with a root CA pool")
		}
		if tlsConfig.InsecureSkipVerify {
			t.Error("InsecureSkipVerify must stay false when only a CA bundle is given")
		}
	})

	t.Run("skip verify disables certificate checks", func(t *testing.T) {
		client, err := buildEndpointHTTPClient(configtypes.EndpointTLSSettings{SkipVerify: true}, "p")
		if err != nil {
			t.Fatalf("buildEndpointHTTPClient() error = %v", err)
		}
		if !client.GetTransport().TLSClientConfig.InsecureSkipVerify {
			t.Error("InsecureSkipVerify = false, want true")
		}
	})
}

func TestBuildEndpointHTTPClient_SharedPerSettings(t *testing.T) {
	ResetEndpointHTTPClients()
	t.Cleanup(ResetEndpointHTTPClients)

	settings := configtypes.EndpointTLSSettings{CABundle: writeSelfSignedPEM(t)}
	first, err := buildEndpointHTTPClient(settings, "a")
	if err != nil {
		t.Fatalf("buildEndpointHTTPClient() error = %v", err)
	}
	second, _ := buildEndpointHTTPClient(settings, "b")
	if first != second {
		t.Error("profiles with the same TLS settings should share one client")
	}
	other, _ := buildEndpointHTTPClient(configtypes.EndpointTLSSettings{SkipVerify: true}, "a")
	if other == first {
		t.Error("different TLS settings must not share a client")
	}

	ResetEndpointHTTPClients()
	rebuilt, _ := buildEndpointHTTPClient(settings, "a")
	if rebuilt == first {
		t.Error("ResetEndpointHTTPClients should force a new client")
	}
}

func TestBuildEndpointHTTPClient_KeepsSDKTLSDefaults(t *testing.T) {
	ResetEndpointHTTPClients()
	t.Cleanup(ResetEndpointHTTPClients)

	client, err := buildEndpointHTTPClient(configtypes.EndpointTLSSettings{SkipVerify: true}, "p")
	if err != nil {
		t.Fatalf("buildEndpointHTTPClient() error = %v", err)
	}
	tlsConfig := client.GetTransport().TLSClientConfig
	if tlsConfig.MinVersion < tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want at least TLS 1.2", tlsConfig.MinVersion)
	}
	if client.GetTransport().Proxy == nil {
		t.Error("the SDK proxy-from-environment setting must be kept")
	}
}
