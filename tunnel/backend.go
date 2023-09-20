package tunnel

import (
	"context"
	"errors"
	"io"
	"nat/message"
	"net"

	"github.com/sirupsen/logrus"
)

type BackendTunnel struct {
	*Proxy
	localAddr string
}

func NewBackendTunnel(localAddr string, remoteAddr *net.UDPAddr, conn *net.UDPConn) *BackendTunnel {
	t := &BackendTunnel{
		localAddr: localAddr,
		Proxy:     NewProxy(remoteAddr, conn),
	}
	return t
}

func (t *BackendTunnel) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr":  t.localAddr,
		"remoteAddr": t.raddr.String(),
	})
	log.Infof("start backend tunnel")
	defer log.Infof("backend tunnel exit.")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		err := t.RunProxy(ctx)
		if err != nil {
			log.Errorf("proxy error:%v", err)
			cancel()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-t.errCh:
			log.Errorf("proxy recv error:%v", err)
			if errors.Is(err, net.ErrClosed) {
				return err
			}
		case msg, ok := <-t.tch:
			if !ok && msg == nil {
				logrus.Info("proxy msg chan closed!")
				return nil
			}
			ch := make(chan message.PacketMessage, 10)
			go func(msg *message.TunnelMessage) {
				err := t.handle(ctx, msg.Id, ch)
				if err != nil {
					log.Errorf("handle backend session %d error:%v", msg.Id, err)
				}
			}(msg)
		}
	}
}

func (t *BackendTunnel) handle(ctx context.Context, id uint64, msgChan chan message.PacketMessage) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"id":         id,
		"localAddr":  t.localAddr,
		"remoteAddr": t.raddr.String(),
	})
	log.Infof("start backend tunnel handle")
	defer log.Infof("backend tunnel hanle exit.")

	conn, err := net.Dial("tcp", t.localAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	udp := t.Proxy.ReadWriter(id)
	go func() {
		defer cancel()
		_, err := io.Copy(conn, udp)
		if err != nil {
			log.Error("local copy to proxy error:%v", err)
		}

	}()
	go func() {
		defer cancel()
		_, err := io.Copy(udp, conn)
		if err != nil {
			log.Error("proxy copy to local error:%v", err)
		}
	}()
	<-ctx.Done()
	return ctx.Err()
}
