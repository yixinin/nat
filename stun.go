package main

import (
	"context"
	"encoding/json"
	"nat/stderr"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type StunClient struct {
	Addr     *net.UDPAddr
	Sdp      Sdp
	UpdateAt time.Time
}
type Stun struct {
	ttl       time.Duration
	localAddr *net.UDPAddr
	clients   map[uint64]StunClient
}

func NewStun(addr string) (*Stun, error) {
	localAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return &Stun{
		ttl:       time.Minute,
		localAddr: localAddr,
		clients:   make(map[uint64]StunClient),
	}, nil
}

func (s *Stun) Run(ctx context.Context) error {
	conn, err := net.ListenUDP("udp", s.localAddr)
	if err != nil {
		return stderr.Wrap(err)
	}

	tk := time.NewTicker(5 * time.Second)
	defer tk.Stop()

	var buf = make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			for k := range s.clients {
				if s.clients[k].UpdateAt.Add(s.ttl).Before(time.Now()) {
					delete(s.clients, k)
				}
			}
		default:
			err := s.handle(ctx, conn, buf)
			if err != nil {
				return stderr.Wrap(err)
			}
		}
	}
}

func (s *Stun) handle(ctx context.Context, conn *net.UDPConn, buf []byte) error {
	err := conn.SetReadDeadline(time.Now().Add(time.Second))
	if err != nil {
		return stderr.Wrap(err)
	}
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		if os.IsTimeout(err) {
			return nil
		}
		return stderr.Wrap(err)
	}

	conn.WriteToUDP(Encode(Message{Type: TypeEcho}), addr)
	msg, err := Decode(buf[:n])
	if err != nil {
		return stderr.Wrap(err)
	}
	logrus.Debugf("recv msg: %s %s", addr, msg)

	switch msg.Type {
	case TypeEcho:
	case TypeSdp:
		var sdp Sdp
		err := json.Unmarshal(msg.Payload, &sdp)
		if err != nil {
			return stderr.Wrap(err)
		}

		// send other client to current
		for k, v := range s.clients {
			if k != msg.ClientId {
				sdp := RemoteSdp{
					Addr: v.Addr,
					Sdp:  v.Sdp,
				}
				data, err := json.Marshal(sdp)
				if err != nil {
					return stderr.Wrap(err)
				}
				msg := NewMessage(k, TypeSdp, data)
				_, err = conn.WriteToUDP(Encode(msg), addr)
				if err != nil {
					return stderr.Wrap(err)
				}
				logrus.Debugf("send sdp2 msg:%s %s", addr, msg)
			}
		}
		s.clients[msg.ClientId] = StunClient{
			Addr:     addr,
			UpdateAt: time.Now(),
			Sdp:      sdp,
		}
		// send current to other
		{
			data, err := json.Marshal(RemoteSdp{
				Addr: addr,
				Sdp:  sdp,
			})
			if err != nil {
				return stderr.Wrap(err)
			}
			msg := NewMessage(msg.ClientId, TypeSdp, data)
			for k, v := range s.clients {
				if k != msg.ClientId {
					_, err = conn.WriteToUDP(Encode(msg), v.Addr)
					if err != nil {
						return stderr.Wrap(err)
					}
					logrus.Debugf("send sdp2 msg:%s %s", v.Addr, msg)
				}
			}

		}

	}
	return nil
}
