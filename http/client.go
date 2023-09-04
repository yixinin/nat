package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"nat/stderr"
	"net"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/sirupsen/logrus"
)

type Client struct {
	conn  *net.UDPConn
	raddr *net.UDPAddr
	hc    *http.Client
}

func NewClient(conn *net.UDPConn, raddr *net.UDPAddr) (*Client, error) {
	c := &Client{
		conn:  conn,
		raddr: raddr,
	}
	roundTripper := &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		QuicConfig: &quic.Config{},
	}
	hclient := &http.Client{
		Transport: roundTripper,
	}
	c.hc = hclient
	return c, nil
}

func (c *Client) Run(ctx context.Context) error {
	logrus.Debug("run http3 quic client")
	tk := time.NewTicker(5 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			rsp, err := c.hc.Get(fmt.Sprintf("https://%s:%d/hello", c.raddr.IP.String(), c.raddr.Port))
			if err != nil {
				return stderr.Wrap(err)
			}
			defer rsp.Body.Close()

			data, err := io.ReadAll(rsp.Body)
			if err != nil {
				return stderr.Wrap(err)
			}
			logrus.Debugf("get hello: %s", data)
		}
	}
}
