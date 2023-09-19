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

	chRw := sync.RWMutex{}
	pkgChs := make(map[uint64]chan message.PacketMessage, 8)
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

			ch := make(chan message.PacketMessage, 10)
			chRw.Lock()
			pkgChs[msg.Id] = ch
			chRw.Unlock()
			go func(msg message.TunnelMessage) {
				if err := t.handle(ctx, sessid.Load(), conn, ch); err != nil {
					log.WithField("id", sessid.Load()).Errorf("handle proxy error:%v", err)
				}

				chRw.Lock()
				ch, ok := pkgChs[msg.Id]
				if ok && ch != nil {
					close(ch)
				}
				delete(pkgChs, msg.Id)
				chRw.Unlock()

			}(msg)
		case msg, ok := <-t.msgCh:
			if !ok && msg == nil {
				log.Info("accept chan closed!")
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
			}
		}
	}
}

func (t *FrontendTunnel) handle(ctx context.Context, id uint64, conn net.Conn, msgCh chan message.PacketMessage) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"id":    id,
		"laddr": t.localAddr,
	})
	log.Debug("start handle")
	defer log.Debug("handle exit.")
	var errCh = make(chan error, 1)
	defer close(errCh)

	lpc := make(chan []byte, 1)
	defer close(lpc)
	defer conn.Close()

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
				continue
			}
			return stderr.Wrap(err)
		case msg, ok := <-msgCh:
			if !ok && msg.Id == 0 {
				log.Info("proxy chan closed!")
				return nil
			}
			n, err := conn.Write(msg.Data)
			if err != nil {
				return err
			}
			log.Debugf("write local data:%d", n)

		case data, ok := <-lpc:
			if !ok && len(data) == 0 {
				log.Info("local lpc chan closed!")
				return nil
			}
			msgs := message.NewPacketMessage(id, seq.Load(), data)
			for i := range msgs {
				t.Proxy.SendMessage(&msgs[i])
				seq.Add(1)
			}
		}
	}
}
