package http

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

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

	go func() {
		if err := s.Sync(ctx); err != nil {
			logrus.Errorf("run sync error:%v", err)
		}
	}()
	return server.ListenAndServeTLS("quic.crt", "quic.key")
}

func (s *TcpServer) Sync(ctx context.Context) error {
	var sync = func() {
		resp, err := http.Get("http://6.ipw.cn")
		if err != nil {
			logrus.Errorf("get self ipv6 error:%v", err)
			return
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logrus.Errorf("get self ipv6 error:%v", err)
			return
		}
		vals := url.Values{
			"name": []string{"opi"},
			"addr": []string{string(data)},
		}
		var s = "http://114.115.218.1:8080/dns?" + vals.Encode()
		logrus.Debug(s)
		_, err = http.Get(s)
		if err != nil {
			logrus.Errorf("post self ipv6 error:%v", err)
			return
		}
	}
	sync()
	tk := time.NewTicker(time.Minute)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			sync()
		}
	}
}
