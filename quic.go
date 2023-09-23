package main

import (
	"context"
	"crypto/tls"

	"github.com/gin-gonic/gin"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type QuicServer struct {
}

func (s *QuicServer) Run(ctx context.Context) error {
	ca, err := tls.LoadX509KeyPair("ca/quic.iakl.top.pem", "ca/quic.iakl.top.key")
	if err != nil {
		return err
	}

	e := gin.Default()

	e.GET("/hello", func(c *gin.Context) {
		c.String(200, "hello, quic!")
	})

	e.NoRoute(func(c *gin.Context) {
		c.String(200, "hello, quic anywhere!")
	})

	var server = http3.Server{
		Addr: ":444",
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{ca},
		},
		QuicConfig: &quic.Config{},
		Handler:    e,
	}
	return server.ListenAndServe()
}
