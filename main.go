package main

import (
	"context"
	"flag"
	"nat/stun"
	"os"
	"os/signal"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/sirupsen/logrus"
)

var (
	debug  bool
	config string
)

func main() {
	flag.BoolVar(&debug, "debug", true, "debug mode")
	flag.StringVar(&config, "c", "config.yaml", "config file")
	flag.Parse()

	{
		logger := &lumberjack.Logger{
			Filename:   "nat.log",
			MaxSize:    500, // megabytes
			MaxBackups: 3,
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		}
		logrus.SetOutput(logger)
	}

	logrus.SetLevel(logrus.InfoLevel)
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.SetFormatter(&logrus.TextFormatter{})

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	config, err := LoadConfig(config)
	if err != nil {
		logrus.Error(err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Run(ctx, config)
	}()

	var ch = make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-ch:
			cancel()
		}
	}
}

func Run(ctx context.Context, config *Config) {
	switch {
	case config.Server != nil:
		c := config.Server
		if config.Server.DDNS {
			go func() {
				err := NewDdns().Run(ctx)
				if err != nil {
					logrus.Errorf("run ddns error:%v", err)
				}
			}()
		}
		s, err := stun.NewServer(c.Addr)
		if err != nil {
			logrus.Errorf("new stun server error:%v", err)
			return
		}
		err = s.Run(ctx)
		if err != nil {
			logrus.Errorf("run stun server error:%v", err)
		}
	case config.Backend != nil:
		if config.Quic != nil {
			go func() {
				err := (&QuicServer{
					config: config.Quic,
				}).Run(ctx)
				logrus.Errorf("run quic http3 server error:%v", err)
			}()
		}
		b, err := NewBackend(config.Backend)
		if err != nil {
			logrus.Errorf("new backend server error:%v", err)
		}
		err = b.Run(ctx)
		if err != nil {
			logrus.Errorf("run backend error:%v", err)
		}
	case config.Frontend != nil:
		f := NewFrontend(config.Frontend)
		err := f.Run(ctx)
		if err != nil {
			logrus.Errorf("run frontend error:%v", err)
		}
	}
}
