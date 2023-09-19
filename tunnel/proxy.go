package tunnel

import (
	"context"
	"nat/message"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

type Proxy struct {
	rconn *net.UDPConn
	raddr *net.UDPAddr

	errCh  chan error
	msgCh  chan message.MessageUnmarshal
	sendCh chan message.Message
}

func NewProxy(remoteAddr *net.UDPAddr, conn *net.UDPConn) *Proxy {
	return &Proxy{
		rconn: conn,
		raddr: remoteAddr,

		errCh: make(chan error, 1),
		msgCh: make(chan message.MessageUnmarshal, 1),

		sendCh: make(chan message.Message, 10),
	}
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
	defer close(p.msgCh)

	go p.loop(ctx)

	buf := make([]byte, message.BufferSize)
	for {
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
		p.msgCh <- msg
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
