package main

import (
	"context"
	"nat/stderr"
	"nat/stun"
	"nat/tunnel"
	"sync/atomic"

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
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": b.localAddr,
	})
	log.Infof("run backend server ")
	defer log.Infof("backend server exit.")

	count := atomic.Int32{}

	go func() {
		count.Add(1)
		b.accept(ctx)
		count.Add(-1)
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-b.stun.NewAccept():
			logrus.Debugf("accept new conn...")
			if count.Load() >= 2 {
				continue
			}
			go func() {
				count.Add(1)
				b.accept(ctx)
				count.Add(-1)
			}()
		}
	}
}

func (b *Backend) accept(ctx context.Context) {
	defer func() {
		b.stun.NewAccept() <- struct{}{}
	}()
	conn, raddr, err := b.stun.Accept(ctx)
	if err != nil {
		logrus.WithContext(ctx).Errorf("accept error:%v", err)
		return
	}

	t := tunnel.NewBackendTunnel(b.localAddr, raddr, conn)
	err = t.Run(ctx)
	if err != nil {
		logrus.WithContext(ctx).Errorf("run tunnel error:%v", err)
	}
}
