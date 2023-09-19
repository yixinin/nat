package tunnel

import (
	"context"
	"net"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

type FrontendTunnel struct {
	localAddr  string
	remoteAddr *net.UDPAddr
	proxy      *net.UDPConn
	raddr      *net.UDPAddr
}

func NewFrontendTunnel(localAddr string, remoteAddr *net.UDPAddr, proxy *net.UDPConn) *FrontendTunnel {
	t := &FrontendTunnel{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		proxy:      proxy,
	}
	return t
}

func (t *FrontendTunnel) Run(ctx context.Context) error {
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.remoteAddr.String(),
	}).Infof("frontend tunnel exit.")

	lis, err := net.Listen("tcp", t.localAddr)
	if err != nil {
		return err
	}
	defer lis.Close()

	var connCh = make(chan net.Conn, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithContext(ctx).WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(connCh)
		}()
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		connCh <- conn
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case conn, ok := <-connCh:
			if !ok {
				return nil
			}
			go func() {
				if err := t.handle(ctx, conn); err != nil {
					logrus.WithContext(ctx).Errorf("handle proxy error:%v", err)
				}
			}()
		}
	}
}

func (t *FrontendTunnel) handle(ctx context.Context, conn net.Conn) error {
	var errCh = make(chan error, 1)
	defer close(errCh)

	lpc := make(chan []byte, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithContext(ctx).WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(lpc)
		}()
		var buf = make([]byte, 1500)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			lpc <- buf[:n]
		}
	}()

	rpc := make(chan []byte, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithContext(ctx).WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(rpc)
		}()
		var buf = make([]byte, 1500)
		for {
			n, err := t.proxy.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			rpc <- buf[:n]
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case data, ok := <-lpc:
			if !ok {
				return nil
			}
			_, err := t.proxy.WriteToUDP(data, t.raddr)
			if err != nil {
				return err
			}
		case data, ok := <-rpc:
			if !ok {
				return nil
			}
			_, err := conn.Write(data)
			if err != nil {
				return err
			}
		case err := <-errCh:
			return err
		}
	}
}
