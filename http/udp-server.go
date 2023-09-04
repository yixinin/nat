package http

import (
	"context"
	"crypto/tls"
	"nat/stderr"
	"net"

	"github.com/gin-gonic/gin"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/sirupsen/logrus"
)

type UDPServer struct {
	conn   *net.UDPConn
	engine *gin.Engine
}

func NewUDPServer(conn *net.UDPConn) (*UDPServer, error) {
	s := &UDPServer{
		conn:   conn,
		engine: gin.Default(),
	}
	s.Init()

	return s, nil
}

func (s *UDPServer) Init() {
	s.engine.GET("/hello", func(c *gin.Context) {
		c.String(200, "hello quic!")
	})
}

func (s *UDPServer) Run(ctx context.Context) error {
	logrus.Debug("run http3 quic server")
	ct, err := tls.LoadX509KeyPair("quic.crt", "quic.key")
	if err != nil {
		return stderr.Wrap(err)
	}
	quicConf := &quic.Config{}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{ct},
	}
	server := http3.Server{
		Handler:    s.engine,
		TLSConfig:  tlsConf,
		QuicConfig: quicConf,
	}
	return server.Serve(s.conn)

}
