package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
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
	var buf = make([]byte, 512)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := conn2.Read(buf)
			if err != nil {
				return err
			}
			fmt.Println("server recv", string(buf[:n]))
		}
	}
}

func client(ctx context.Context) error {
	laddr := &net.UDPAddr{
		Port: 8802,
	}
	raddr := &net.UDPAddr{
		Port: 8888,
		IP:   net.IPv4(114, 115, 218, 1),
	}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP([]byte("client"), raddr)
	if err != nil {
		return err
	}
	var buf = make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	fmt.Println("client recv", string(buf[:n]))
	raddr = new(net.UDPAddr)
	err = json.Unmarshal(buf[:n], raddr)
	if err != nil {
		return err
	}
	fmt.Println("try to connect", raddr)

	n, err = conn.WriteToUDP([]byte("hello server, I'm client from stun"), raddr)
	fmt.Println(n, err)
	return err
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
			fmt.Println(string(buf[:n]), raddr)
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
			}
		}
	}
}
