package stream

import (
	"context"
	"crypto/tls"
	"nat/stderr"
	"net"
	"os"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
)

type Client struct {
	conn *net.UDPConn
	lis  *quic.Listener
}

func (c *Client) Run(ctx context.Context) error {
	quicConf := &quic.Config{}
	ct, err := tls.LoadX509KeyPair("quic.crt", "quic.key")
	if err != nil {
		return stderr.Wrap(err)
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{ct},
	}
	lis, err := quic.Listen(c.conn, tlsConf, quicConf)
	if err != nil {
		return stderr.Wrap(err)
	}
	c.lis = lis
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			err := func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				conn, err := c.lis.Accept(ctx)
				if err != nil {
					if os.IsTimeout(err) {
						return nil
					}
					return stderr.Wrap(err)
				}
				go func(ctx context.Context, conn quic.Connection) {
					if err := c.Serve(ctx, conn); err != nil {
						logrus.Errorf("serve conn error:%v", err)
					}
				}(ctx, conn)
				return nil
			}(ctx)
			if err != nil {
				return stderr.Wrap(err)
			}
		}
	}
}

func (c *Client) Serve(ctx context.Context, conn quic.Connection) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				stream, err := conn.AcceptStream(ctx)
				if err != nil {
					if os.IsTimeout(err) {
						return nil
					}
					return stderr.Wrap(err)
				}
				go func(ctx context.Context, stream quic.Stream) {
					if err := c.ServeStream(ctx, stream); err != nil {
						logrus.Errorf("serve stream error:%v", err)
					}
				}(ctx, stream)
				return nil
			}(ctx)
			if err != nil {
				return stderr.Wrap(err)
			}
		}
	}
}

func (c *Client) ServeStream(ctx context.Context, stream quic.Stream) error {
	var buf = make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := stream.SetReadDeadline(time.Now().Add(time.Second))
			if err != nil {
				return stderr.Wrap(err)
			}
			n, err := stream.Read(buf)
			if err != nil {
				return stderr.Wrap(err)
			}
			go func() {
				err := c.handle(ctx, buf[:n])
				if err != nil {
					logrus.Errorf("handle error:%v", err)
				}
			}()

		}
	}
}

func (c *Client) handle(ctx context.Context, buf []byte) error {
	logrus.Info(string(buf))
	return nil
}
