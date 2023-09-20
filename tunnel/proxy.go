package tunnel

import (
	"context"
	"errors"
	"io"
	"nat/message"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type Proxy struct {
	sync.RWMutex
	rconn *net.UDPConn
	raddr *net.UDPAddr

	errCh   chan error
	recvChs map[uint64]chan message.MessageUnmarshal
	tch     chan *message.TunnelMessage
	sendCh  chan message.Message
}

func (p *Proxy) AddRecvCh(id uint64, ch chan message.MessageUnmarshal) {
	p.Lock()
	defer p.Unlock()
	p.recvChs[id] = ch
}
func (p *Proxy) GetRecvCh(id uint64) chan message.MessageUnmarshal {
	p.RLock()
	defer p.RUnlock()
	return p.recvChs[id]
}
func (p *Proxy) DelRecvCh(id uint64) {
	p.Lock()
	defer p.Unlock()
	ch, ok := p.recvChs[id]
	if ok && ch != nil {
		close(ch)
	}
	delete(p.recvChs, id)
}

func NewProxy(remoteAddr *net.UDPAddr, conn *net.UDPConn) *Proxy {
	return &Proxy{
		rconn: conn,
		raddr: remoteAddr,

		errCh:   make(chan error, 1),
		recvChs: make(map[uint64]chan message.MessageUnmarshal, 8),
		tch:     make(chan *message.TunnelMessage, 1),
		sendCh:  make(chan message.Message, 10),
	}
}

type rw struct {
	cancel context.CancelFunc
	seq    atomic.Uint64
	id     uint64
	recvCh chan message.MessageUnmarshal
	sendCH chan message.Message
}

func (rw *rw) Read(buf []byte) (int, error) {
	for {
		msg, ok := <-rw.recvCh
		if !ok || msg == nil {
			return 0, io.ErrClosedPipe
		}
		data, ok := msg.(*message.PacketMessage)
		if !ok {
			continue
		}
		if len(data.Data) > len(buf) {
			return 0, errors.New("buffer too small")
		}
		return copy(buf, data.Data), nil
	}

}
func (rw *rw) Write(buf []byte) (int, error) {
	msgs := message.NewPacketMessage(rw.id, rw.seq.Load(), buf)
	for i := range msgs {
		rw.sendCH <- &msgs[i]
	}
	return len(buf), nil
}
func (rw *rw) Close() error {
	if rw.cancel != nil {
		rw.cancel()
	}
	return nil
}
func (p *Proxy) ReadWriter(id uint64) io.ReadWriteCloser {
	recvCh := make(chan message.MessageUnmarshal, 10)
	p.AddRecvCh(id, recvCh)

	rw := &rw{
		id:     id,
		sendCH: p.sendCh,
		recvCh: recvCh,
	}
	rw.cancel = func() {
		p.DelRecvCh(id)
	}
	return rw
}

func (p *Proxy) SendMessage(msg ...message.Message) {
	for _, msg := range msg {
		p.sendCh <- msg
	}
}

func (p *Proxy) RunProxy(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"remoteAddr": p.raddr.String(),
	})
	log.Infof("start proxy")
	defer log.Infof("proxy exit.")
	defer close(p.errCh)

	go p.loop(ctx)

	buf := make([]byte, message.BufferSize)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, raddr, err := p.rconn.ReadFromUDP(buf)
		if err != nil {
			p.errCh <- err
			continue
		}

		if n == 0 {
			log.Debug("recv empty data")
			continue
		}
		if raddr.String() != p.raddr.String() {
			log.Debug("recv unkown data")
			continue
		}
		msg, err := message.Unmarshal(buf[:n])
		if err != nil {
			p.errCh <- err
			continue
		}
		log.Debugf("recv proxy msg:%s size:%d", msg.Type(), n)
		switch msg := msg.(type) {
		case *message.PacketMessage:
			ch := p.GetRecvCh(msg.Id)
			if ch != nil {
				ch <- msg
			} else {
				log.Debugf("channel closed! drop proxy msg:%s size:%d", msg.Type(), n)
			}
		case *message.TunnelMessage:
			if p.tch != nil {
				p.tch <- msg
			}
		case *message.HeartbeatMessage:
			if !msg.NoRelay {
				msg.NoRelay = true
				p.SendMessage(msg)
			}
		case *message.HandShakeMessage:
			if !msg.NoRelay {
				msg.NoRelay = true
				p.SendMessage(msg)
			}
		default:
			log.Debugf("drop proxy msg:%s size:%d", msg.Type(), n)
		}
	}
}

func (p *Proxy) loop(ctx context.Context) {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"raddr": p.raddr.String(),
	})
	log.Info("start proxy loop")
	defer log.Info("proxy loop exit.")
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			msg := message.HeartbeatMessage{
				NoRelay: true,
			}
			p.SendMessage(&msg)
		case msg := <-p.sendCh:
			data, err := message.Marshal(msg)
			if err != nil {
				logrus.Error(err)
				continue
			}
			n, err := p.rconn.WriteToUDP(data, p.raddr)
			if err != nil {
				log.WithFields(logrus.Fields{
					"raddr": p.raddr.String(),
				}).Debugf("send proxy msg:%s error:%v", msg.Type(), err)
			} else {
				log.WithFields(logrus.Fields{
					"raddr": p.raddr.String(),
				}).Debugf("send proxy msg:%s size:%d", msg.Type(), n)
			}

		}
	}
}
