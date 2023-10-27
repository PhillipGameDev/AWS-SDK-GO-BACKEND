// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package awsv1shim

import (
	"net/http"
	"testing"

	"github.com/hashicorp/aws-sdk-go-base/v2/internal/config"
	"github.com/hashicorp/aws-sdk-go-base/v2/internal/test"
)

func TestHTTPClientConfiguration_basic(t *testing.T) {
	client, err := defaultHttpClient(&config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Unexpected type for HTTP client transport: %T", client.Transport)
	}

	test.HTTPClientConfigurationTest_basic(t, transport)
}

func TestHTTPClientConfiguration_insecureHTTPS(t *testing.T) {
	client, err := defaultHttpClient(&config.Config{
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Unexpected type for HTTP client transport: %T", client.Transport)
	}

	test.HTTPClientConfigurationTest_insecureHTTPS(t, transport)
}

func TestHTTPClientConfiguration_proxy(t *testing.T) {
	test.HTTPClientConfigurationTest_proxy(t, transport)
}

func transport(t *testing.T, config *config.Config) *http.Transport {
	t.Helper()

	client, err := defaultHttpClient(config)
	if err != nil {
		t.Fatalf("creating client: %s", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Unexpected type for HTTP client transport: %T", client.Transport)
	}

	return transport
}
