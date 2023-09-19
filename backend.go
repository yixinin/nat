package main

import (
	"context"
	"nat/stderr"
	"nat/stun"
	"nat/tunnel"

	"github.com/sirupsen/logrus"
)

type Backend struct {
	localAddr string
	stun      *stun.Backend
}

func NewBackend(fqdn, stunAddr string) (*Backend, error) {
	stun, err := stun.NewBackend(fqdn, stunAddr)
	if err != nil {
		return nil, stderr.Wrap(err)
	}
	return &Backend{
		localAddr: localAddr,
		stun:      stun,
	}, nil
}

func (b *Backend) Run(ctx context.Context) error {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr": b.localAddr,
	}).Infof("run backend server ")

	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr": b.localAddr,
	}).Infof("backend server exit.")
	go b.accept(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-b.stun.NewAccept():
			logrus.Debugf("accept new conn...")
			go b.accept(ctx)
		}
	}
}

func (b *Backend) accept(ctx context.Context) {
	conn, raddr, err := b.stun.Accept(ctx)
	if err != nil {
		logrus.WithContext(ctx).Errorf("accept error:%v", err)
		b.stun.NewAccept() <- struct{}{}
		return
	}

	t := tunnel.NewBackendTunnel(b.localAddr, raddr, conn)
	err = t.Run(ctx)
	if err != nil {
		logrus.WithContext(ctx).Errorf("run tunnel error:%v", err)
	}
}
