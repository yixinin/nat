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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, nil, err
	}

	var syncStun = func() error {
		msg := message.StunMessage{
			ClientType: message.Backend,
		}
		data, err := message.Marshal(msg)
		if err != nil {
			return err
		}
		_, err = conn.WriteToUDP(data, b.stunAddr)
		return err
	}

	if err := syncStun(); err != nil {
		return nil, nil, err
	}

	var tk = time.NewTicker(10 * time.Second)
	defer tk.Stop()

	once := sync.Once{}
	var buf = make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-tk.C:
			if err := syncStun(); err != nil {
				return nil, nil, err
			}
		default:
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, raddr, err := conn.ReadFromUDP(buf)
			if os.IsTimeout(err) {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			if n == 0 {
				continue
			}

			msg, err := message.Unmarshal(buf[:n])
			if err != nil {
				return nil, nil, err
			}
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
				return conn, raddr, nil
			}
		}
	}
}

func handshake(ctx context.Context, conn *net.UDPConn, raddr *net.UDPAddr) error {
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			hsMsg := message.HandShakeMessage{}
			data, err := message.Marshal(hsMsg)
			if err != nil {
				return err
			}
			_, err = conn.WriteToUDP(data, raddr)
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

func (f *Frontend) Dial(ctx context.Context, addr string) (*net.UDPConn, *net.UDPAddr, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, nil, err
	}

	var dialStun = func() error {
		msg := message.StunMessage{
			ClientType: message.Frontend,
			FQDN:       addr,
		}
		data, err := message.Marshal(msg)
		if err != nil {
			return err
		}
		_, err = conn.WriteToUDP(data, f.stunAddr)
		return err
	}

	if err := dialStun(); err != nil {
		return nil, nil, err
	}

	tk := time.NewTicker(3 * time.Second)
	defer tk.Stop()

	var buf = make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, err
		case <-tk.C:
			if err := dialStun(); err != nil {
				return nil, nil, err
			}
		default:
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, raddr, err := conn.ReadFromUDP(buf)
			if os.IsTimeout(err) {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			if n == 0 {
				continue
			}

			msg, err := message.Unmarshal(buf[:n])
			if err != nil {
				return nil, nil, err
			}
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
				return conn, raddr, nil
			}
		}
	}
}
