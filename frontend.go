package main

import (
	"context"
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

func NewFrontend(localAddr, stunAddr, fqdn string) *Frontend {
	f, err := stun.NewFrontend(stunAddr)
	if err != nil {
		return nil
	}
	return &Frontend{
		localAddr: localAddr,
		fqdn:      fqdn,
		stun:      f,
	}
}

func (f *Frontend) Run(ctx context.Context) error {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr": f.localAddr,
		"fqdn":      f.fqdn,
	}).Info("start frontend")

	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr": f.localAddr,
		"fqdn":      f.fqdn,
	}).Info("frontend exit.")
	conn, raddr, err := f.stun.Dial(ctx, f.fqdn)
	if err != nil {
		return err
	}
	t := tunnel.NewFrontendTunnel(f.localAddr, raddr, conn)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	return t.Run(ctx)
}
