package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"nat/stun"
	"nat/tunnel"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Frontend struct {
	sync.RWMutex
	localAddr string
	fqdns     []string
	tunnels   map[string]*tunnel.FrontendTunnel
	stun      *stun.Frontend
}

func NewFrontend(c *FrontendConfig) *Frontend {
	f, err := stun.NewFrontend(c.StunAddr)
	if err != nil {
		return nil
	}
	return &Frontend{
		localAddr: c.Addr,
		fqdns:     c.FQDN,
		stun:      f,
		tunnels:   make(map[string]*tunnel.FrontendTunnel, 2),
	}
}

func (f *Frontend) AddTunnel(fqdn string, t *tunnel.FrontendTunnel) {
	f.Lock()
	defer f.Unlock()

	f.tunnels[fqdn] = t
}
func (f *Frontend) DelTunnel(fqdn string) {
	f.Lock()
	defer f.Unlock()
	delete(f.tunnels, fqdn)
}
func (f *Frontend) GetTunnel(fqdn string) (*tunnel.FrontendTunnel, bool) {
	t, ok := f.tunnels[fqdn]
	return t, ok
}

func (f *Frontend) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": f.localAddr,
		"fqdn":  f.fqdns,
	})
	log.Info("start frontend")

	defer log.Info("frontend exit.")
	dctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	lis, err := net.Listen("tcp", f.localAddr)
	if err != nil {
		return err
	}

	var connCh = make(chan tunnel.Lconn, 1)

	go func() {
		var buf [1204]byte
		for {
			conn, err := lis.Accept()
			if err != nil {
				log.Errorf("local accept error:%v", err)
				cancel()
				return
			}

			n, err := conn.Read(buf[:])
			if err != nil {
				log.Errorf("local accept read error:%v", err)
				continue
			}
			var method, URL, address string
			// 从客户端数据读入 method，url
			_, err = fmt.Sscanf(string(buf[:bytes.IndexByte(buf[:n], '\n')]), "%s%s", &method, &URL)
			if err != nil {
				log.Errorf("local accept read parse error:%v", err)
				conn.Close()
				continue
			}
			hostPortURL, err := url.Parse(URL)
			if err != nil {
				log.Errorf("local accept read parse url error:%v", err)
				conn.Close()
				return
			}

			var fqdn = strings.Split(hostPortURL.Host, ":")[0]
			if method == "CONNECT" {
				address = hostPortURL.Scheme + ":" + hostPortURL.Opaque
			}
			connCh <- tunnel.NewLconn(conn, buf[:n], fqdn, address)
			log.Infof("accept conn: %s", fqdn)
		}

	}()

	var wg sync.WaitGroup
	for i := range f.fqdns {
		wg.Add(1)
		go func(fqdn string) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				conn, raddr, err := f.stun.Dial(dctx, fqdn)
				if err != nil {
					log.Errorf("dial error:%v", err)
					return
				}
				if conn == nil || raddr == nil {
					return
				}

				t := tunnel.NewFrontendTunnel(fqdn, raddr, conn)
				f.AddTunnel(fqdn, t)
				err = t.Run(ctx)
				f.DelTunnel(fqdn)
				if errors.Is(err, tunnel.ErrorTunnelClosed) {
					continue
				}
				if err != nil {
					log.Errorf("dial error:%v", err)
					return
				}
			}

		}(f.fqdns[i])
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case lconn, ok := <-connCh:
		if !ok && lconn.FQDN == "" {
			log.Error("local accept exited.")
			return nil
		}

		t, ok := f.GetTunnel(lconn.FQDN)
		if ok {
			t.Accept(lconn)
		} else {
			log.Errorf("fqdn:%s not found.", lconn.FQDN)
			_, err := fmt.Fprint(lconn.Conn, "HTTP/1.1 502 serve not found\r\n\r\n")
			if err != nil {
				log.Errorf("write back https connected error:%v", err)
				return err
			}
		}
	}
	wg.Wait()
	return nil
}
