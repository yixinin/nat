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
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.remoteAddr.String(),
	}).Infof("start backend tunnel")
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
