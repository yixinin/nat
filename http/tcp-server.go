package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/sirupsen/logrus"
)

type TcpServer struct {
}

func NewTcpServer() *TcpServer {
	return &TcpServer{}
}

func (s *TcpServer) Run(ctx context.Context) error {
	e := gin.Default()
	e.GET("/hello", func(c *gin.Context) {
		c.String(200, "hello from ipv6")
	})

	logrus.Debug("run http3 quic server")

	quicConf := &quic.Config{}

	server := http3.Server{
		Handler:    e,
		Addr:       ":8080",
		QuicConfig: quicConf,
	}

	return server.ListenAndServeTLS("quic.crt", "quic.key")
}
