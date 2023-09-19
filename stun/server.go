package stun

import (
	"context"
	"nat/message"
	"net"
	"os"

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

type RemoteData struct {
	addr *net.UDPAddr
	data []byte
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

	var errCh = make(chan error, 1)
	defer close(errCh)

	var dataCh = make(chan RemoteData, 1)

	go func() {
		defer close(dataCh)
		var buf = make([]byte, 1500)
		for {
			n, raddr, err := conn.ReadFromUDP(buf)
			if os.IsTimeout(err) {
				continue
			}
			if err != nil {
				errCh <- err
				return
			}
			if n == 0 {
				continue
			}
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": raddr.String(),
				"laddr": s.localAddr,
			}).Debugf("recved %d data", n)
			dataCh <- RemoteData{
				addr: raddr,
				data: buf[:n],
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case item, ok := <-dataCh:
			if !ok {
				return nil
			}
			var remoteIP = item.addr.IP.String()

			msg, err := message.Unmarshal(item.data)
			if err != nil {
				return nil
			}
			m, ok := msg.(*message.StunMessage)
			if !ok {
				return nil
			}
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"raddr": item.addr.String(),
				"laddr": s.localAddr,
			}).Debugf("recved data:%v", msg)
			switch m.ClientType {
			case message.Backend:
				if err := s.dns.SetIP(ctx, m.FQDN, remoteIP); err != nil {
					return err
				}

				if err := s.dns.SetIpAddr(ctx, remoteIP, item.addr); err != nil {
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
					targetAddr, err = s.dns.GetPairAddr(ctx, item.addr)
				}
				if err != nil || targetAddr == nil {
					return err
				}

				// send to backend target
				{
					var msg = message.ConnMessage{
						RemoteAddr: item.addr,
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
					body, err := message.Marshal(msg)
					if err != nil {
						return err
					}
					_, err = conn.WriteToUDP(body, item.addr)
					if err != nil {
						return err
					}
				}

			}

			if err != nil {
				logrus.WithContext(ctx).Error(err)
			}
		}
	}
}
