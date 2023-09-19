package stun

import (
	"context"
	"fmt"
	"nat/storage"
	"nat/storage/mem"
	"net"
	"time"
)

type Dns struct {
	db storage.Storage
}

func NewDns() *Dns {
	return &Dns{
		db: mem.NewMemCahce(),
	}
}

func (Dns) akey(fqnd string) string {
	return fmt.Sprintf("dns/fqnd/%s", fqnd)
}
func (Dns) ipKey(ip string) string {
	return fmt.Sprintf("/stun/ip/%s", ip)
}
func (Dns) pairKey(addr *net.UDPAddr) string {
	return fmt.Sprintf("/stun/pair/%s", addr.String())
}

func (d *Dns) GetIP(ctx context.Context, fqdn string) (string, error) {
	val, err := d.db.Get(ctx, d.akey(fqdn))
	if err == mem.ErrorNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	ip, _ := val.(string)
	return ip, nil
}
func (d *Dns) SetIP(ctx context.Context, fqdn string, ip string) error {
	return d.db.Set(ctx, d.akey(fqdn), ip, 10*time.Minute)
}

func (d *Dns) GetIPAddr(ctx context.Context, ip string) (*net.UDPAddr, error) {
	val, err := d.db.Get(ctx, d.ipKey(ip))
	if err == mem.ErrorNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	addr, ok := val.(*net.UDPAddr)
	if addr == nil || !ok {
		return nil, nil
	}
	d.db.Delete(ctx, d.ipKey(ip))
	return addr, nil
}

func (d *Dns) SetIpAddr(ctx context.Context, ip string, addr *net.UDPAddr) error {
	return d.db.Set(ctx, d.ipKey(ip), addr, 30*time.Second)
}

func (d *Dns) GetPairAddr(ctx context.Context, key *net.UDPAddr) (*net.UDPAddr, error) {
	val, err := d.db.Get(ctx, d.pairKey(key))
	if err == mem.ErrorNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	addr, _ := val.(*net.UDPAddr)
	return addr, nil
}

func (d *Dns) SetPairAddr(ctx context.Context, key, addr *net.UDPAddr) error {
	return d.db.Set(ctx, d.pairKey(key), addr, 30*time.Second)
}
