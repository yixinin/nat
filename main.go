package main

import (
	"context"
	"flag"
	"fmt"
	"nat/http"

	"github.com/sirupsen/logrus"
)

var (
	stun   bool
	server bool
	client bool
	port   int
	tcp    bool
	dns    bool
)

func main() {
	flag.BoolVar(&stun, "t", false, "stun server")
	flag.BoolVar(&server, "s", false, "http server")
	flag.BoolVar(&client, "c", false, "http client")
	flag.IntVar(&port, "p", 8888, "local addr")
	flag.BoolVar(&tcp, "tcp", false, "listen tcp")
	flag.BoolVar(&dns, "dns", false, "listen dns")
	flag.Parse()

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{})

	var stunAddr = "114.115.218.1:8888"
	var localAddr = fmt.Sprintf(":%d", port)
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	switch {
	case tcp:
		err := http.NewTcpServer().Run(ctx)
		if err != nil {
			logrus.Errorf("run tcp error:%v", err)
		}
	case dns:
		err := NewDdns().Run(ctx)
		if err != nil {
			logrus.Errorf("run ddns error:%v", err)
		}
	case stun:
		s, err := NewStun(localAddr)
		if err != nil {
			logrus.Errorf("new stun error:%v", err)
			return
		}
		err = s.Run(ctx)
		if err != nil {
			logrus.Errorf("run stun error:%v", err)
		}
	case client:
		c, err := NewClient(TypeClient, localAddr, stunAddr)
		if err != nil {
			logrus.Errorf("new client.client error:%v", err)
			return
		}
		err = c.Run(ctx)
		if err != nil {
			logrus.Errorf("run client.client error:%v", err)
		}
	case server:
		c, err := NewClient(TypeServer, localAddr, stunAddr)
		if err != nil {
			logrus.Errorf("new client.server error:%v", err)
			return
		}
		err = c.Run(ctx)
		if err != nil {
			logrus.Errorf("run client.server error:%v", err)
		}
	}
}
