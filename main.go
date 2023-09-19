package main

import (
	"context"
	"flag"
	"nat/stun"

	"github.com/sirupsen/logrus"
)

var (
	stunServer bool
	ddns       bool
	backend    bool
	frontend   bool
	stunAddr   string
	localAddr  string
	fqdn       string
)

func main() {
	flag.BoolVar(&stunServer, "s", false, "stun server")
	flag.BoolVar(&backend, "b", false, "backend server")
	flag.BoolVar(&frontend, "f", false, "frontend cient")
	flag.StringVar(&stunAddr, "stun", "114.115.218.1:8080", "stun server addr")
	flag.StringVar(&localAddr, "localAddr", "", "listen addr")
	flag.StringVar(&fqdn, "fqdn", "", "fqdn")
	flag.Parse()

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{})

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	switch {
	case stunServer:
		if ddns {
			go func() {
				err := NewDdns().Run(ctx)
				if err != nil {
					logrus.Errorf("run ddns error:%v", err)
				}
			}()
		}
		s, err := stun.NewServer(localAddr)
		if err != nil {
			logrus.Errorf("run stun server error:%v", err)
			return
		}
		err = s.Run(ctx)
		if err != nil {
			logrus.Errorf("run stun server error:%v", err)
		}
	case backend:

	case frontend:
		f := NewFrontend(localAddr, stunAddr, fqdn)
		err := f.Run(ctx)
		if err != nil {
			logrus.Errorf("run stun error:%v", err)
		}
	}
}
