package stun

import (
	"context"
	"nat/message"
	"net"
	"os"
	"runtime/debug"
	"time"

	"github.com/sirupsen/logrus"
)

type Backend struct {
	FQDN     string
	stunAddr *net.UDPAddr

	newAccept chan struct{}
}

func NewBackend(fqdn, stunAddr string) (*Backend, error) {
	addr, err := net.ResolveUDPAddr("udp", stunAddr)
	if err != nil {
		return nil, err
	}
	return &Backend{
		FQDN:     fqdn,
		stunAddr: addr,

		newAccept: make(chan struct{}, 10),
	}, nil
}

func (b *Backend) NewAccept() chan struct{} {
	return b.newAccept
}

func (b *Backend) Accept(ctx context.Context) (*net.UDPConn, *net.UDPAddr, error) {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"stunAddr": b.stunAddr.String(),
		"fqdn":     b.FQDN,
	}).Info("start accept")
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"stunAddr": b.stunAddr.String(),
		"fqdn":     b.FQDN,
	}).Info("accept exit.")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, nil, err
	}

	var syncStun = func() error {
		msg := message.NewStunMessage(message.Backend, b.FQDN)
		data, err := message.Marshal(msg)
		if err != nil {
			return err
		}
		n, err := conn.WriteToUDP(data, b.stunAddr)
		logrus.WithContext(ctx).WithFields(logrus.Fields{
			"raddr": b.stunAddr.String(),
		}).Debugf("send %d data:%v", n, msg)
		return err
	}

	if err := syncStun(); err != nil {
		return nil, nil, err
	}

	var tk = time.NewTicker(10 * time.Second)
	defer tk.Stop()

	var errCh = make(chan error, 1)
	defer close(errCh)
	var dataCh = make(chan RemoteData, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithContext(ctx).WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(dataCh)
		}()
		var buf = make([]byte, 1024)
		for {
			n, raddr, err := conn.ReadFromUDP(buf)
			if os.IsTimeout(err) {
				logrus.WithContext(ctx).Debug("read timeout")
				continue
			}
			if err != nil {
				errCh <- err
				return
			}
			if n == 0 {
				logrus.WithContext(ctx).WithFields(logrus.Fields{
					"raddr": raddr,
				}).Debug("read no data")
				continue
			}
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": raddr.String(),
				"fqdn":  b.FQDN,
			}).Debugf("recved %d data", n)
			dataCh <- RemoteData{
				addr: raddr,
				data: buf[:n],
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-tk.C:
			if err := syncStun(); err != nil {
				return nil, nil, err
			}
		case d, ok := <-dataCh:
			if !ok {
				return nil, nil, nil
			}
			msg, err := message.Unmarshal(d.data)
			if err != nil {
				return nil, nil, err
			}
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": d.addr.String(),
				"fqdn":  b.FQDN,
			}).Debugf("recved data:%v", msg)
			switch msg := msg.(type) {
			case *message.ConnMessage:
				tk.Stop()
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				err := handshake(ctx, conn, msg.RemoteAddr)
				if os.IsTimeout(err) {
					logrus.WithContext(ctx).Errorf("handshake with %s timeout, will retry in %d seconds", msg.RemoteAddr, 10)
					tk.Reset(10 & time.Second)
					continue
				}
				if err != nil {
					logrus.WithContext(ctx).Errorf("handshake with %s error:%v", msg.RemoteAddr, err)
					return nil, nil, err
				}
				if err == nil {
					b.newAccept <- struct{}{}
				}
			case *message.HandShakeMessage:
				// received handshake, success.
				return conn, d.addr, nil
			}
		}
	}
}
