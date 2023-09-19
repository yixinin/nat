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
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.remoteAddr.String(),
	}).Infof("start frontend tunnel")
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
		var buf = make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			logrus.WithContext(ctx).Debugf("recv %d local data", n)
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
		var buf = make([]byte, 1024)
		for {
			n, raddr, err := t.proxy.ReadFromUDP(buf)
			if err != nil {
				errCh <- err
				return
			}
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": raddr.String(),
			}).Debugf("recv %d data", n)
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
			n, err := t.proxy.WriteToUDP(data, t.raddr)
			if err != nil {
				return err
			}
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": t.raddr.String(),
			}).Debugf("send %d data", n)
		case data, ok := <-rpc:
			if !ok {
				return nil
			}
			n, err := conn.Write(data)
			if err != nil {
				return err
			}
			logrus.WithContext(ctx).Debugf("send %d local data", n)
		case err := <-errCh:
			return err
		}
	}
}
