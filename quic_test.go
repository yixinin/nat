package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

func TestQuic(t *testing.T) {
	roundTripper := &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		QuicConfig: &quic.Config{},
	}
	defer roundTripper.Close()

	hc := http.Client{
		Transport: roundTripper,
	}
	resp, err := hc.Get("https://[2409:8a28:8a1:eb50::624]:8080/hello")
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(string(data))
}
