package main

import (
	"context"
	"nat/stderr"
	"nat/stun"
	"nat/tunnel"
	"time"

	"github.com/sirupsen/logrus"
)

type Frontend struct {
	localAddr string
	fqdn      string

	stun *stun.Frontend
}

func NewFrontend(c *FrontendConfig) *Frontend {
	f, err := stun.NewFrontend(c.StunAddr)
	if err != nil {
		return nil
	}
	return &Frontend{
		localAddr: c.Addr,
		fqdn:      c.FQDN,
		stun:      f,
	}
}

func (f *Frontend) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": f.localAddr,
		"fqdn":  f.fqdn,
	})
	log.Info("start frontend")

	defer log.Info("frontend exit.")
	dctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	conn, raddr, err := f.stun.Dial(dctx, f.fqdn)
	if err != nil {
		return stderr.Wrap(err)
	}
	t := tunnel.NewFrontendTunnel(f.fqdn, f.localAddr, raddr, conn)

	return t.Run(ctx)
}
