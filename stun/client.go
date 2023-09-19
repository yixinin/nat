package stun

import (
	"context"
	"errors"
	"nat/message"
	"net"
	"os"
	"sync"
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

	once := sync.Once{}

	var errCh = make(chan error, 1)
	defer close(errCh)
	var dataCh = make(chan RemoteData, 1)

	go func() {
		defer close(dataCh)
		var buf = make([]byte, 1500)
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
				once.Do(func() {
					b.newAccept <- struct{}{}
					go func() {
						err := handshake(ctx, conn, msg.RemoteAddr)
						if err != nil && !errors.Is(err, ctx.Err()) {
							logrus.WithContext(ctx).Errorf("handshake with %s error:%v", msg.RemoteAddr, err)
							cancel()
						}
					}()
				})

			case *message.HandShakeMessage:
				// received handshake, success.
				cancel()
				return conn, d.addr, nil
			}
		}
	}
}

func handshake(ctx context.Context, conn *net.UDPConn, raddr *net.UDPAddr) error {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"remoteAddr": raddr.String(),
	}).Info("start handshake")
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"remoteAddr": raddr.String(),
	}).Info("handshake exit.")
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			msg := message.HandShakeMessage{}
			data, err := message.Marshal(msg)
			if err != nil {
				return err
			}

			n, err := conn.WriteToUDP(data, raddr)
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": raddr.String(),
			}).Debugf("send %d data:%v", n, msg)
			if err != nil {
				return err
			}
		}
	}
}

type Frontend struct {
	stunAddr *net.UDPAddr
}

func NewFrontend(stunAddr string) (*Frontend, error) {
	addr, err := net.ResolveUDPAddr("udp", stunAddr)
	if err != nil {
		return nil, err
	}
	return &Frontend{stunAddr: addr}, nil
}

func (f *Frontend) Dial(ctx context.Context, fqdn string) (*net.UDPConn, *net.UDPAddr, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, nil, err
	}

	var dialStun = func() error {
		msg := message.NewStunMessage(message.Frontend, fqdn)
		data, err := message.Marshal(msg)
		if err != nil {
			return err
		}
		n, err := conn.WriteToUDP(data, f.stunAddr)
		logrus.WithContext(ctx).WithFields(logrus.Fields{
			"raddr": f.stunAddr.String(),
		}).Debugf("send %d data:%v", n, msg)
		return err
	}

	if err := dialStun(); err != nil {
		return nil, nil, err
	}

	tk := time.NewTicker(3 * time.Second)
	defer tk.Stop()

	var errCh = make(chan error, 1)
	defer close(errCh)
	var dataCh = make(chan RemoteData, 1)

	go func() {
		defer close(dataCh)
		var buf = make([]byte, 1500)
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
				"fqdn":  fqdn,
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
			return nil, nil, err
		case <-tk.C:
			if err := dialStun(); err != nil {
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
				"fqdn":  fqdn,
			}).Debugf("recved data:%v", msg)
			switch msg := msg.(type) {
			case *message.ConnMessage:
				go func() {
					err := handshake(ctx, conn, msg.RemoteAddr)
					if err != nil && !errors.Is(err, ctx.Err()) {
						logrus.WithContext(ctx).Errorf("handshake with %s error:%v", msg.RemoteAddr, err)
						cancel()
					}
				}()
			case *message.HandShakeMessage:
				// received handshake, success.
				cancel()
				return conn, d.addr, nil
			}
		}
	}
}
