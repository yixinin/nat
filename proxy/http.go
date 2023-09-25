package proxy

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type HttpProxy struct {
	Addr string
}

func (p *HttpProxy) Run(ctx context.Context) error {
	lis, err := net.Listen("tcp", p.Addr)
	if err != nil {
		return err
	}
	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}
		go func() {
			err := p.handle(ctx, conn)
			if err != nil {
				logrus.Errorf("hanle proxy error:%v", err)
			}
		}()
	}
}
func (p *HttpProxy) handle(ctx context.Context, conn net.Conn) error {
	var b [1024]byte
	//从客户端获取数据
	n, err := conn.Read(b[:])
	if err != nil {
		return err
	}

	var method, URL, address string
	fmt.Sscanf(string(b[:bytes.IndexByte(b[:n], '\n')]), "%s%s", &method, &URL)
	hostPortURL, err := url.Parse(URL)
	if err != nil {
		return err
	}
	if method == "CONNECT" {
		address = hostPortURL.Scheme + ":" + hostPortURL.Opaque
	} else { //否则为 http 协议
		address = hostPortURL.Host
		// 如果 host 不带端口，则默认为 80
		if strings.Index(hostPortURL.Host, ":") == -1 { //host 不带端口， 默认 80
			address = hostPortURL.Host + ":80"
		}
	}
	if method == "CONNECT" {
		fmt.Fprint(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
	} else { //如果使用 http 协议，需将从客户端得到的 http 请求转发给服务端
		server.Write(b[:n])
	}
	return ctx.Err()
}
