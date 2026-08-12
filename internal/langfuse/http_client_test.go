package langfuse

import (
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The httptest certificate covers the DNS name example.com as well as the
// loopback addresses, so a request to the server's 127.0.0.1 URL would pass on
// the IP alone. Overriding the verified name to something the certificate does
// not cover is what proves the override is the name being checked.
func newTestTLSServer(t *testing.T) (*httptest.Server, *x509.CertPool) {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())

	return server, roots
}

func trustRoots(t *testing.T, client *http.Client, roots *x509.CertPool) {
	t.Helper()

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected an *http.Transport, got %T", client.Transport)
	}
	transport.TLSClientConfig.RootCAs = roots
}

func TestNewHTTPClientWithoutServerNameKeepsDefaults(t *testing.T) {
	if transport := newHTTPClient("").Transport; transport != nil {
		t.Fatalf("expected the default transport, got %T", transport)
	}
}

func TestNewHTTPClientVerifiesAgainstServerName(t *testing.T) {
	server, roots := newTestTLSServer(t)

	client := newHTTPClient("example.com")
	trustRoots(t, client, roots)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestNewHTTPClientRejectsMismatchedServerName(t *testing.T) {
	server, roots := newTestTLSServer(t)

	client := newHTTPClient("not-in-the-certificate.example")
	trustRoots(t, client, roots)

	resp, err := client.Get(server.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected certificate verification to fail for a name the certificate does not cover")
	}
}

func TestHTTPClientOrDefault(t *testing.T) {
	if httpClientOrDefault(nil) == nil {
		t.Fatal("expected a usable client for a nil argument")
	}

	client := &http.Client{}
	if httpClientOrDefault(client) != client {
		t.Fatal("expected the provided client to be returned unchanged")
	}
}
