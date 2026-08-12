package langfuse

import (
	"crypto/tls"
	"net/http"
)

// newHTTPClient builds the HTTP client shared by the API clients.
//
// When tlsServerName is set, the certificate is verified against that name
// instead of the host in the request URL, and that name is sent as the SNI
// value. This is what allows an instance to be reached through a port forward
// or tunnel, where the URL host is something like localhost:8080 and can never
// match the certificate. Verification itself stays enabled.
func newHTTPClient(tlsServerName string) *http.Client {
	if tlsServerName == "" {
		return &http.Client{}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		ServerName: tlsServerName,
		MinVersion: tls.VersionTLS12,
	}

	return &http.Client{Transport: transport}
}

func httpClientOrDefault(httpClient *http.Client) *http.Client {
	if httpClient == nil {
		return &http.Client{}
	}

	return httpClient
}
