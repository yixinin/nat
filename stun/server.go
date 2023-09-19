package stun

import (
	"context"
	"nat/message"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type Server struct {
	dns *Dns

	localAddr *net.UDPAddr
}

func NewServer(localAddr string) (*Server, error) {
	s := &Server{
		dns: NewDns(),
	}
	addr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return nil, err
	}
	s.localAddr = addr
	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr": s.localAddr.String(),
	}).Infof("start stun server")
	defer logrus.WithContext(ctx).WithFields(logrus.Fields{
		"localAddr": s.localAddr.String(),
	}).Infof("stun server exit.")

	conn, err := net.ListenUDP("udp", s.localAddr)
	if err != nil {
		return err
	}
	var buf = make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			err := func() error {
				n, raddr, err := conn.ReadFromUDP(buf)
				if os.IsTimeout(err) {
					return nil
				}
				if err != nil {
					return err
				}
				if n == 0 {
					return nil
				}
				var remoteIP = raddr.IP.String()

				msg, err := message.Unmarshal(buf[:n])
				if err != nil {
					return nil
				}
				m, ok := msg.(*message.StunMessage)
				if !ok {
					return nil
				}
				logrus.WithContext(ctx).WithFields(logrus.Fields{
					"raddr": raddr.String(),
					"laddr": s.localAddr,
				}).Debugf("recved data:%v", msg)
				switch m.ClientType {
				case message.Backend:
					if err := s.dns.SetIP(ctx, m.FQDN, remoteIP); err != nil {
						return err
					}

					if err := s.dns.SetIpAddr(ctx, remoteIP, raddr); err != nil {
						return err
					}
				case m.ClientType:
					targetIP, err := s.dns.GetIP(ctx, m.FQDN)
					if err != nil {
						return err
					}
					if targetIP == "" {
						return nil
					}

					targetAddr, err := s.dns.GetIPAddr(ctx, targetIP)
					if err != nil {
						return err
					}

					if targetAddr == nil {
						targetAddr, err = s.dns.GetPairAddr(ctx, raddr)
					}
					if err != nil || targetAddr == nil {
						return err
					}

					// send to backend target
					{
						var msg = message.ConnMessage{
							RemoteAddr: raddr,
						}
						data, err := message.Marshal(msg)
						if err != nil {
							return err
						}
						_, err = conn.WriteToUDP(data, targetAddr)
						if err != nil {
							return err
						}
					}

					// send back to frontend
					{
						var msg = message.ConnMessage{
							RemoteAddr: targetAddr,
						}
						data, err := message.Marshal(msg)
						if err != nil {
							return err
						}
						_, err = conn.WriteToUDP(data, raddr)
						if err != nil {
							return err
						}
					}

				}
				return nil
			}()

			if err != nil {
				logrus.WithContext(ctx).Error(err)
			}
		}
	}
}
