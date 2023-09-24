package stun

import (
	"context"
	"nat/message"
	"nat/stderr"
	"net"
	"os"
	"runtime/debug"
	"time"

	"github.com/sirupsen/logrus"
)

type Backend struct {
	FQDN     string
	stunAddr *net.UDPAddr
}

func NewBackend(fqdn, stunAddr string) (*Backend, error) {
	addr, err := net.ResolveUDPAddr("udp", stunAddr)
	if err != nil {
		return nil, stderr.Wrap(err)
	}
	return &Backend{
		FQDN:     fqdn,
		stunAddr: addr,
	}, nil
}

func (b *Backend) Accept(ctx context.Context) (*net.UDPConn, *net.UDPAddr, error) {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"stunAddr": b.stunAddr.String(),
		"fqdn":     b.FQDN,
	})
	log.Info("start accept")
	defer log.Info("accept exit.")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, nil, stderr.Wrap(err)
	}

	var syncStun = func() error {
		msg := message.NewStunMessage(message.Backend, b.FQDN)
		data, err := message.Marshal(msg)
		if err != nil {
			return stderr.Wrap(err)
		}
		n, err := conn.WriteToUDP(data, b.stunAddr)
		log.WithFields(logrus.Fields{
			"raddr": b.stunAddr.String(),
		}).Debugf("send %d data:%v", n, msg)
		return stderr.Wrap(err)
	}

	if err := syncStun(); err != nil {
		return nil, nil, stderr.Wrap(err)
	}

	var tk = time.NewTicker(10 * time.Second)
	defer tk.Stop()

	var errCh = make(chan error, 1)
	defer close(errCh)
	var dataCh = make(chan RemoteData, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(dataCh)
		}()
		var buf = make([]byte, message.BufferSize)
		for {
			n, raddr, err := conn.ReadFromUDP(buf)
			if os.IsTimeout(err) {
				log.Debug("read timeout")
				continue
			}
			if err != nil {
				errCh <- err
				return
			}
			if n == 0 {
				log.WithFields(logrus.Fields{
					"raddr": raddr,
				}).Debug("read no data")
				continue
			}
			dataCh <- RemoteData{
				addr: raddr,
				data: buf[:n],
			}
		}
	}()
	var handshakeCanel context.CancelFunc = func() {}
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-tk.C:
			if err := syncStun(); err != nil {
				return nil, nil, stderr.Wrap(err)
			}
		case d, ok := <-dataCh:
			if !ok && d.data == nil {
				return nil, nil, nil
			}
			msg, err := message.Unmarshal(d.data)
			if err != nil {
				return nil, nil, stderr.Wrap(err)
			}
			log.WithFields(logrus.Fields{
				"raddr": d.addr.String(),
				"fqdn":  b.FQDN,
			}).Debugf("recv data:%s", msg.Type())
			switch msg := msg.(type) {
			case *message.ConnMessage:
				tk.Stop()
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				handshakeCanel = cancel
				defer cancel()
				err := handShakeTick(ctx, conn, msg.RemoteAddr)
				if os.IsTimeout(err) {
					log.Errorf("handshake with %s timeout, will retry in %d seconds", msg.RemoteAddr, 10)
					tk.Reset(10 * time.Second)
					continue
				}
				if err != nil {
					log.Errorf("handshake with %s error:%v", msg.RemoteAddr, err)
					return nil, nil, stderr.Wrap(err)
				}
			case *message.HandShakeMessage:
				handshakeCanel()
				time.Sleep(10 * time.Millisecond)
				data, _ := message.Marshal(message.ReadyMessage{})
				_, err := conn.WriteToUDP(data, d.addr)
				if err != nil {
					return nil, nil, stderr.Wrap(err)
				}
			case *message.ReadyMessage:
				return conn, d.addr, nil
			}
		}
	}
}
