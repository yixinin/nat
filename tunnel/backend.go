package tunnel

import (
	"context"
	"errors"
	"io"
	"nat/message"
	"nat/stderr"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

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

	chRw := sync.RWMutex{}
	pkgChs := make(map[uint64]chan message.PacketMessage, 8)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-t.errCh:
			log.Errorf("proxy recv error:%v", err)
			if errors.Is(err, net.ErrClosed) {
				return err
			}
		case msg, ok := <-t.msgCh:
			if !ok && msg == nil {
				logrus.Info("proxy msg chan closed!")
				return nil
			}
			switch msg := msg.(type) {
			case *message.PacketMessage:
				if msg == nil {
					log.Info("msg is nil")
					continue
				}
				chRw.RLock()
				ch, ok := pkgChs[msg.Id]
				if ok && ch != nil {
					ch <- *msg
				} else {
					log.Info("channel is nil ", msg.Id, ok, ch == nil)
				}
				chRw.RUnlock()
			case *message.TunnelMessage:
				ch := make(chan message.PacketMessage, 10)
				chRw.Lock()
				pkgChs[msg.Id] = ch
				chRw.Unlock()
				go func(msg *message.TunnelMessage) {
					err := t.handle(ctx, msg.Id, ch)
					if err != nil {
						log.Errorf("backend proxy handle %d error:%v", msg.Id, err)
					}
					chRw.Lock()
					ch, ok := pkgChs[msg.Id]
					if ok && ch != nil {
						close(ch)
					}
					delete(pkgChs, msg.Id)
					chRw.Unlock()

				}(msg)
			case *message.HeartbeatMessage:
				if !msg.NoRelay {
					msg.NoRelay = true
					t.Proxy.SendMessage(msg)
				}
			case *message.HandShakeMessage:
				if !msg.NoRelay {
					msg.NoRelay = true
					t.Proxy.SendMessage(msg)
				}
			default:
				log.Errorf("unexpected recv %s msg", msg.Type())
			}
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

	var lpc = make(chan []byte, 1)
	var errCh = make(chan error, 1)
	defer close(lpc)
	defer close(errCh)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
		}()
		var buf = make([]byte, message.BufferSize)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := conn.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			log.Debugf("recv local %d data", n)
			lpc <- buf[:n]
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	exit := time.NewTimer(time.Second)
	exit.Stop()

	seq := atomic.Uint64{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exit.C:
			return nil
		case err := <-errCh:
			if errors.Is(err, io.EOF) {
				log.Info("disconnected wait flush data")
				// exit.Reset(10 * time.Second)
				continue
			}
			return stderr.Wrap(err)
		case msg, ok := <-msgChan:
			if !ok && msg.Id == 0 {
				log.Info("proxy chan closed!")
				return nil
			}
			n, err := conn.Write(msg.Data)
			if err != nil {
				return err
			}
			log.Debugf("write local data:%d", n)
		case data := <-lpc:
			msgs := message.NewPacketMessage(id, seq.Load(), data)
			for i := range msgs {
				t.Proxy.SendMessage(&msgs[i])
				seq.Add(1)
			}
		}
	}
}
