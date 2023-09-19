package tunnel

import (
	"context"
	"nat/message"
	"nat/stderr"
	"net"
	"runtime/debug"
	"time"

	"github.com/sirupsen/logrus"
)

type BackendTunnel struct {
	localAddr string

	proxy *net.UDPConn
	raddr *net.UDPAddr
}

func NewBackendTunnel(localAddr string, remoteAddr *net.UDPAddr, proxy *net.UDPConn) *BackendTunnel {
	t := &BackendTunnel{
		localAddr: localAddr,
		raddr:     remoteAddr,
		proxy:     proxy,
	}
	return t
}

func (t *BackendTunnel) Run(ctx context.Context) error {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.raddr.String(),
	}).Infof("start backend tunnel")
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.raddr.String(),
	}).Infof("backend tunnel exit.")
	conn, err := net.Dial("tcp", t.localAddr)
	if err != nil {
		return stderr.Wrap(err)
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
		var buf = make([]byte, message.BufferSize)
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
		var buf = make([]byte, message.BufferSize)
		for {
			n, raddr, err := t.proxy.ReadFromUDP(buf)
			if err != nil {
				errCh <- err
				return
			}
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": raddr.String(),
			}).Debugf("recv %d data", n)
			msg, err := message.Unmarshal(buf[:n])
			if err != nil {
				errCh <- err
				return
			}
			switch msg := msg.(type) {
			case *message.PacketMessage:
				rpc <- msg.Data
			case *message.HeartbeatMessage:
				if !msg.NoRelay {
					msg.NoRelay = true
					data, _ := message.Marshal(msg)
					t.proxy.WriteToUDP(data, raddr)
				}
			case *message.HandShakeMessage:
				if !msg.NoRelay {
					msg.NoRelay = true
					data, _ := message.Marshal(msg)
					t.proxy.WriteToUDP(data, raddr)
				}
			}

		}
	}()

	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			msg := message.HeartbeatMessage{}
			data, _ := message.Marshal(msg)
			t.proxy.WriteToUDP(data, t.raddr)
		case data, ok := <-lpc:
			if !ok {
				logrus.Debug("local lpc chan closed!")
				return nil
			}
			msgs := message.NewPacketMessage(data)
			for _, msg := range msgs {
				data, _ = message.Marshal(msg)
				n, err := t.proxy.WriteToUDP(data, t.raddr)
				if err != nil {
					return stderr.Wrap(err)
				}
				logrus.WithContext(ctx).WithFields(logrus.Fields{
					"raddr": t.raddr.String(),
				}).Debugf("send %d data", n)
			}
		case data, ok := <-rpc:
			if !ok {
				logrus.Debug("local rpc chan closed!")
				return nil
			}
			n, err := conn.Write(data)
			if err != nil {
				return stderr.Wrap(err)
			}
			logrus.WithContext(ctx).Debugf("send %d local data", n)
		case err := <-errCh:
			return stderr.Wrap(err)
		}
	}
}
