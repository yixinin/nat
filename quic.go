package main

import (
	"context"
	"crypto/tls"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type QuicServer struct {
}

func (s *QuicServer) Run(ctx context.Context) error {
	ca, err := tls.LoadX509KeyPair("quic.crt", "quic.key")
	if err != nil {
		return err
	}

	var server = http3.Server{
		Addr: ":444",
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{ca},
		},
		QuicConfig: &quic.Config{},
	}
	return server.ListenAndServe()
}
