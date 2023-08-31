package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"
)

var (
	Type string
	lp   int
)

func main() {
	flag.StringVar(&Type, "t", "", "run type")
	flag.IntVar(&lp, "p", 8888, "local addr")
	flag.Parse()

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	switch Type {
	case "stun":
		err := stun(ctx)
		fmt.Println(err)
	case "client":
		err := client(ctx)
		fmt.Println(err)
	}
}

func client(ctx context.Context) error {
	laddr := &net.UDPAddr{
		Port: lp,
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	raddr := &net.UDPAddr{
		Port: 8888,
		IP:   net.IPv4(114, 115, 218, 1),
	}
	n, err := conn.WriteToUDP([]byte(strconv.Itoa(lp)), raddr)
	fmt.Println("send to stun", raddr, n, err)
	go func(raddr *net.UDPAddr) {
		tk := time.NewTicker(time.Second)
		for range tk.C {
			n, err := conn.WriteToUDP([]byte("::"), raddr)
			fmt.Println("send to stun", raddr, n, err)
		}
	}(raddr)

	var buf = make([]byte, 512)
	for {
		n, raddr, err = conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		fmt.Println("client recv", raddr, string(buf[:n]))
		switch n {
		case 2:
		case 0:
		default:
			switch buf[0] {
			case '{':
				raddr = new(net.UDPAddr)
				err = json.Unmarshal(buf[:n], raddr)
				if err != nil {
					return err
				}
				fmt.Println("try to connect", raddr)
				go func(raddr *net.UDPAddr) {
					tk := time.NewTicker(time.Second)
					for range tk.C {
						n, err = conn.WriteToUDP([]byte("hello server, I'm client from stun"), raddr)
						fmt.Println("send to server", raddr, n, err)
					}
				}(raddr)
			default:
			}
		}
	}
}

func stun(ctx context.Context) error {
	laddr := &net.UDPAddr{
		Port: 8888,
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return err
	}
	var m = map[string]*net.UDPAddr{}
	for {
		var buf = make([]byte, 512)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, raddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return err
			}
			fmt.Println("recv", string(buf[:n]), "from", raddr)

			switch s := string(buf[:n]); s {
			case "::":
				_, err = conn.WriteTo(buf[:n], raddr)
				if err != nil {
					return err
				}
			default:
				if len(m) == 1 {
					for _, addr := range m {
						data, err := json.Marshal(raddr)
						if err != nil {
							return err
						}
						_, err = conn.WriteTo(data, addr)
						if err != nil {
							return err
						}

						data, err = json.Marshal(addr)
						if err != nil {
							return err
						}
						_, err = conn.WriteTo(data, raddr)
						if err != nil {
							return err
						}
					}
				}
				m[string(buf[:n])] = raddr

			}
		}
	}
}
