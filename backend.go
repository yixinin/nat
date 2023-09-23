package main

import (
	"context"
	"nat/stderr"
	"nat/stun"
	"nat/tunnel"
	"os"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type Backend struct {
	laddr             string
	stun              *stun.Backend
	certFile, keyFile string
}

func NewBackend(c *BackendConfig) (*Backend, error) {
	stun, err := stun.NewBackend(c.FQDN, c.StunAddr)
	if err != nil {
		return nil, stderr.Wrap(err)
	}
	return &Backend{
		laddr:    c.Addr,
		stun:     stun,
		certFile: c.CertFile,
		keyFile:  c.KeyFile,
	}, nil
}

func (b *Backend) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": b.laddr,
	})
	log.Infof("run backend server ")
	defer log.Infof("backend server exit.")

	count := atomic.Int32{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if count.Load() >= 2 {
				continue
			}
			count.Add(1)
			go func() {
				defer count.Add(-1)
				b.accept(ctx)
			}()
		}
	}
}

func (b *Backend) accept(ctx context.Context) (ok bool) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	conn, raddr, err := b.stun.Accept(ctx)
	if os.IsTimeout(err) {
		return false
	}
	if err != nil {
		logrus.WithContext(ctx).Errorf("accept error:%v", err)
		return false
	}

	go func() {
		t := tunnel.NewBackendTunnel(b.laddr, raddr, conn, b.certFile, b.keyFile)
		err = t.Run(ctx)
		if err != nil {
			logrus.WithContext(ctx).Errorf("run tunnel error:%v", err)
		}
	}()
	return true
}
