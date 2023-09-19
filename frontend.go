package main

import (
	"context"
	"nat/stun"
	"nat/tunnel"
	"time"
)

type Frontend struct {
	localAddr string
	fqdn      string

	stun *stun.Frontend
}

func NewFrontend(localAddr, strunAddr, fqdn string) *Frontend {
	f, err := stun.NewFrontend(strunAddr)
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
	conn, raddr, err := f.stun.Dial(ctx, f.fqdn)
	if err != nil {
		return err
	}
	t := tunnel.NewFrontendTunnel(f.localAddr, raddr, conn)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	return t.Run(ctx)
}
