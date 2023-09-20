package tunnel

import (
	"context"
	"errors"
	"io"
	"nat/message"
	"nat/stderr"
	"net"
	"runtime/debug"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

type FrontendTunnel struct {
	*Proxy
	localAddr string
}

func NewFrontendTunnel(localAddr string, remoteAddr *net.UDPAddr, conn *net.UDPConn) *FrontendTunnel {
	t := &FrontendTunnel{
		localAddr: localAddr,
		Proxy:     NewProxy(remoteAddr, conn),
	}
	return t
}

func (t *FrontendTunnel) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.raddr.String(),
	})
	log.Infof("start frontend tunnel")
	defer log.Infof("frontend tunnel exit.")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		err := t.RunProxy(ctx)
		if err != nil {
			log.Errorf("proxy error:%v", err)
			cancel()
		}
	}()

	lis, err := net.Listen("tcp", t.localAddr)
	if err != nil {
		return stderr.Wrap(err)
	}
	defer lis.Close()

	var connCh = make(chan net.Conn, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(connCh)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := lis.Accept()
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"laddr": t.localAddr,
				}).Errorf("accept exit with error:%v", err)
				cancel()
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
		case err := <-t.errCh:
			log.Errorf("proxy recv error:%v", err)
			if errors.Is(err, net.ErrClosed) {
				return err
			}
		case conn, ok := <-connCh:
			if !ok && conn == nil {
				log.Debug("accept chan closed!")
				return nil
			}

			msg := message.NewTunnelMessage(sessid.Add(1))
			t.Proxy.SendMessage(&msg)

			go func(msg message.TunnelMessage) {
				if err := t.handle(ctx, sessid.Load(), conn); err != nil {
					log.WithField("id", sessid.Load()).Errorf("handle frontend session error:%v", err)
				}
			}(msg)

		}
	}
}

func (t *FrontendTunnel) handle(ctx context.Context, id uint64, conn net.Conn) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"id":    id,
		"laddr": t.localAddr,
	})
	log.Debug("start handle")
	defer log.Debug("handle exit.")
	defer conn.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	udp := t.Proxy.ReadWriter(id)
	go func() {
		defer cancel()
		n, err := io.Copy(conn, udp)
		if err != nil {
			log.Errorf("local copy to proxy error:%v", err)
		}
		log.Debugf("end local copy %d to proxy", n)
	}()
	go func() {
		defer cancel()
		n, err := io.Copy(udp, conn)
		if err != nil {
			log.Errorf("proxy copy to local error:%v", err)
		}
		log.Debugf("end proxy copy %d to local", n)
	}()
	<-ctx.Done()
	return ctx.Err()
}
