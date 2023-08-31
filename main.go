package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"time"
)

var (
	Type string
)

func main() {
	flag.StringVar(&Type, "t", "", "run type")
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
	case "server":
		err := server(ctx)
		fmt.Println(err)
	}
}

func server(ctx context.Context) error {
	laddr := &net.UDPAddr{
		Port: 8801,
	}
	raddr := &net.UDPAddr{
		Port: 8888,
		IP:   net.IPv4(114, 115, 218, 1),
	}

	conn2, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	_, err = conn2.WriteToUDP([]byte("server"), raddr)
	if err != nil {
		return err
	}
	go func() {
		tk := time.NewTicker(time.Second)
		for range tk.C {
			n, err := conn2.WriteToUDP([]byte("::"), raddr)
			fmt.Println("tick to stun", raddr, n, err)
		}
	}()
	var buf = make([]byte, 512)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, raddr, err := conn2.ReadFromUDP(buf)
			if err != nil {
				return err
			}
			fmt.Println("server recv", raddr, string(buf[:n]))
		}
	}
}

func client(ctx context.Context) error {
	laddr := &net.UDPAddr{
		Port: 8802,
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	raddr := &net.UDPAddr{
		Port: 8888,
		IP:   net.IPv4(114, 115, 218, 1),
	}
	n, err := conn.WriteToUDP([]byte("client"), raddr)
	fmt.Println("send to stun", raddr, n, err)
	go func() {
		raddr := &net.UDPAddr{
			Port: 8888,
			IP:   net.IPv4(114, 115, 218, 1),
		}
		tk := time.NewTicker(time.Second)
		for range tk.C {
			n, err := conn.WriteToUDP([]byte("::"), raddr)
			fmt.Println("send to stun", raddr, n, err)
		}
	}()

	var buf = make([]byte, 512)
	for {
		n, raddr, err = conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		fmt.Println("client recv", raddr, string(buf[:n]))
		switch string(buf[:n]) {
		case "::":
		default:
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
	var serverAddr, clientAddr *net.UDPAddr
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
			data, err := json.Marshal(raddr)
			if err != nil {
				return err
			}
			switch string(buf[:n]) {
			case "server":
				serverAddr = raddr
				if clientAddr != nil {
					_, err = conn.WriteTo(data, clientAddr)
					if err != nil {
						return err
					}
				}
			case "client":
				clientAddr = raddr
				if serverAddr != nil {
					_, err = conn.WriteTo(data, serverAddr)
					if err != nil {
						return err
					}
				}
			default:
				_, err = conn.WriteTo(buf[:n], raddr)
				if err != nil {
					return err
				}
			}
		}
	}
}
