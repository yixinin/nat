package main

import (
	"context"
	"encoding/json"
	"nat/http"
	"nat/stderr"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type ClientType string

const (
	TypeClient ClientType = "client"
	TypeServer ClientType = "server"
)

type Client struct {
	Id        uint64
	Type      ClientType
	localAddr *net.UDPAddr
	conn      *net.UDPConn
	stunAddr  *net.UDPAddr
	OnSdp     chan RemoteSdp
}

func NewClient(typ ClientType, localAddr, stunAddr string) (*Client, error) {
	var c = &Client{
		Id:    uint64(time.Now().Unix()),
		Type:  typ,
		OnSdp: make(chan RemoteSdp),
	}
	addr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return nil, err
	}
	c.localAddr = addr
	addr, err = net.ResolveUDPAddr("udp", stunAddr)
	if err != nil {
		return nil, err
	}
	c.stunAddr = addr
	return c, nil
}

func (c *Client) Run(ctx context.Context) error {
	conn, err := net.ListenUDP("udp", c.localAddr)
	if err != nil {
		return stderr.Wrap(err)
	}
	c.conn = conn

	var sctx, cancel = context.WithCancel(ctx)
	defer cancel()
	go func(ctx context.Context) {
		err := c.Sync(ctx)
		if err != nil {
			logrus.Errorf("run sync error:%v", err)
		}
	}(sctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sdp := <-c.OnSdp:
			cancel()
			if err := c.StartQuic(ctx, sdp); err != nil {
				logrus.Errorf("start quic error:%v", err)
				return nil
			}
		}
	}
}

func (c *Client) StartQuic(ctx context.Context, sdp RemoteSdp) error {
	tk := time.NewTicker(3 * time.Second)
	for range tk.C {
		logrus.Debugf("send echo %s", sdp.Addr)
		_, err := c.conn.WriteToUDP(Encode(Message{Type: TypeEcho}), sdp.Addr)
		if err != nil {
			return stderr.Wrap(err)
		}
		var buf = make([]byte, 512)
		c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, raddr, err := c.conn.ReadFromUDP(buf)
		if err != nil && !os.IsTimeout(err) {
			return stderr.Wrap(err)
		}
		logrus.Debugf("recv echo %s", raddr)
		if n > 0 {
			tk.Stop()
		}
	}

	switch c.Type {
	case TypeServer:
		s, err := http.NewUDPServer(c.conn)
		if err != nil {
			return stderr.Wrap(err)
		}
		return s.Run(ctx)
	case TypeClient:
		c, err := http.NewClient(c.conn, sdp.Addr)
		if err != nil {
			return stderr.Wrap(err)
		}
		return c.Run(ctx)
	}
	return nil
}

func (c *Client) Sync(ctx context.Context) error {
	var tk = time.NewTicker(time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			c.sync(ctx)
		default:
			var buf = make([]byte, 2048)
			err := func(ctx context.Context) error {
				if err := c.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
					return stderr.Wrap(err)
				}
				n, raddr, err := c.conn.ReadFromUDP(buf)
				if err != nil {
					if os.IsTimeout(err) {
						return nil
					}
					return stderr.Wrap(err)
				}

				msg, err := Decode(buf[:n])
				if err != nil {
					return stderr.Wrap(err)
				}
				logrus.Debugf("recv msg:%s %s", raddr, msg)
				switch msg.Type {
				case TypeEcho:
				case TypeSdp:
					var data RemoteSdp
					err := json.Unmarshal(msg.Payload, &data)
					if err != nil {
						return stderr.Wrap(err)
					}
					c.OnSdp <- data
				}
				return nil
			}(ctx)
			if err != nil {
				return stderr.Wrap(err)
			}
		}
	}
}

type Sdp struct {
	Addrs []string
	Port  int
}

type RemoteSdp struct {
	Addr *net.UDPAddr
	Sdp  Sdp `json:"-"`
}

func (c *Client) sync(ctx context.Context) error {
	ns, err := net.Interfaces()
	if err != nil {
		return stderr.Wrap(err)
	}
	var data = Sdp{
		Port: c.localAddr.Port,
	}
	for _, n := range ns {
		addrs, err := n.Addrs()
		if err != nil {
			return stderr.Wrap(err)
		}
		for _, addr := range addrs {
			data.Addrs = append(data.Addrs, addr.String())
		}
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return stderr.Wrap(err)
	}
	var msg = NewMessage(c.Id, TypeSdp, payload)
	_, err = c.conn.WriteToUDP(Encode(msg), c.stunAddr)
	if err != nil {
		return stderr.Wrap(err)
	}
	logrus.Debugf("send msg:%s %s", c.stunAddr, msg)
	return nil
}
