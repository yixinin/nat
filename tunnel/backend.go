package tunnel

import (
	"context"
	"net"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

type BackendTunnel struct {
	localAddr  string
	remoteAddr *net.UDPAddr

	proxy *net.UDPConn
	raddr *net.UDPAddr
}

func NewBackendTunnel(localAddr string, remoteAddr *net.UDPAddr, proxy *net.UDPConn) *BackendTunnel {
	t := &BackendTunnel{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		proxy:      proxy,
	}
	return t
}

func (t *BackendTunnel) Run(ctx context.Context) error {
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.remoteAddr.String(),
	}).Infof("backend tunnel exit.")
	conn, err := net.Dial("tcp", t.localAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

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
