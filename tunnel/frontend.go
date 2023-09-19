package tunnel

import (
	"context"
	"errors"
	"io"
	"nat/message"
	"nat/stderr"
	"net"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type FrontendTunnel struct {
	localAddr string
	proxy     *net.UDPConn
	raddr     *net.UDPAddr
}

func NewFrontendTunnel(localAddr string, remoteAddr *net.UDPAddr, proxy *net.UDPConn) *FrontendTunnel {
	t := &FrontendTunnel{
		localAddr: localAddr,
		raddr:     remoteAddr,
		proxy:     proxy,
	}
	return t
}

func (t *FrontendTunnel) Run(ctx context.Context) error {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.raddr.String(),
	}).Infof("start frontend tunnel")
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.raddr.String(),
	}).Infof("frontend tunnel exit.")

	lis, err := net.Listen("tcp", t.localAddr)
	if err != nil {
		return stderr.Wrap(err)
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
		for {
			conn, err := lis.Accept()
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"laddr": t.localAddr,
				}).Errorf("accept exit with error:%v", err)
				return
			}
			connCh <- conn
		}
	}()

	sessid := atomic.Uint64{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case conn, ok := <-connCh:
			if !ok && conn == nil {
				logrus.WithFields(logrus.Fields{
					"laddr": t.localAddr,
				}).Debug("accept chan closed!")
				return nil
			}
			go func() {
				if err := t.handle(ctx, sessid.Add(1), conn); err != nil {
					logrus.WithContext(ctx).Errorf("handle proxy error:%v", err)
				}
			}()
		}
	}
}

func (t *FrontendTunnel) handle(ctx context.Context, id uint64, conn net.Conn) error {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": t.localAddr,
	}).Debugf("start handle:%d", id)
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": t.localAddr,
	}).Debugf("handle:%d exit.", id)
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
			logrus.WithContext(ctx).WithField("id", id).Debugf("recv local %d data", n)
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
				logrus.WithContext(ctx).Errorf("recv with error:%v", err)
				errCh <- err
				return
			}
			msg, err := message.Unmarshal(buf[:n])
			if err != nil {
				errCh <- err
				return
			}
			switch msg := msg.(type) {
			case *message.PacketMessage:
				logrus.WithContext(ctx).WithFields(logrus.Fields{
					"raddr": raddr.String(),
					"id":    id,
				}).Debugf("sync proxy %d data to send list", len(msg.Data))
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
			default:
				logrus.Errorf("unknown msg:%s", reflect.TypeOf(msg))
			}
		}
	}()

	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if errors.Is(err, io.EOF) {
				logrus.WithContext(ctx).Debug("disconnected wait next session")
				return nil
			}
			return stderr.Wrap(err)
		case <-tk.C:
			msg := message.HeartbeatMessage{}
			data, _ := message.Marshal(msg)
			t.proxy.WriteToUDP(data, t.raddr)
		case data, ok := <-lpc:
			if !ok && len(data) == 0 {
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
					"id":    id,
				}).Debugf("send proxy %d data", n)
			}
		case data, ok := <-rpc:
			if !ok && len(data) == 0 {
				logrus.Debug("local rpc chan closed!")
				return nil
			}
			logrus.WithContext(ctx).WithField("id", id).Debugf("ready send local %d data", len(data))
			n, err := conn.Write(data)
			if err != nil {
				return stderr.Wrap(err)
			}
			logrus.WithContext(ctx).Debugf("send local %d data", n)
		}
	}
}
