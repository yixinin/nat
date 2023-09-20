package stun

import (
	"context"
	"nat/message"
	"nat/stderr"
	"net"
	"os"
	"reflect"
	"runtime/debug"

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
		return nil, stderr.Wrap(err)
	}
	s.localAddr = addr
	return s, nil
}

type RemoteData struct {
	addr *net.UDPAddr
	data []byte
}

func (s *Server) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": s.localAddr.String(),
	})
	log.Infof("start stun server")
	defer log.Infof("stun server exit.")

	conn, err := net.ListenUDP("udp", s.localAddr)
	if err != nil {
		return stderr.Wrap(err)
	}

	var errCh = make(chan error, 1)
	defer close(errCh)

	var dataCh = make(chan RemoteData, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(dataCh)
		}()
		var buf = make([]byte, message.BufferSize)
		for {
			n, raddr, err := conn.ReadFromUDP(buf)
			if os.IsTimeout(err) {
				log.WithFields(logrus.Fields{
					"laddr": s.localAddr,
				}).Debug("read timeout")
				continue
			}
			if err != nil {
				errCh <- err
				return
			}
			if n == 0 {
				log.Debug("read no data")
				continue
			}
			log.WithFields(logrus.Fields{
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
			return stderr.Wrap(err)
		case d, ok := <-dataCh:
			if !ok && d.data == nil {
				log.Debug("data ch closed!")
				return nil
			}
			err := func() error {
				var remoteIP = d.addr.IP.String()
				msg, err := message.Unmarshal(d.data)
				if err != nil {
					return stderr.Wrap(err)
				}
				m, ok := msg.(*message.StunMessage)
				if !ok {
					log.WithFields(logrus.Fields{
						"raddr": d.addr.String(),
						"laddr": s.localAddr,
					}).Debugf("recved unknown data:%s", reflect.TypeOf(msg))
					return nil
				}
				log.Debugf("recv data:%v", msg)
				switch m.ClientType {
				case message.Backend:
					if err := s.dns.SetIP(ctx, m.FQDN, remoteIP); err != nil {
						return stderr.Wrap(err)
					}

					if err := s.dns.SetIpAddr(ctx, remoteIP, d.addr); err != nil {
						return stderr.Wrap(err)
					}
				case m.ClientType:
					targetIP, err := s.dns.GetIP(ctx, m.FQDN)
					if err != nil {
						return stderr.Wrap(err)
					}
					if targetIP == "" {
						return nil
					}

					targetAddr, err := s.dns.GetIPAddr(ctx, targetIP)
					if err != nil {
						return stderr.Wrap(err)
					}

					if targetAddr == nil {
						targetAddr, err = s.dns.GetPairAddr(ctx, d.addr)
					}
					if err != nil {
						return stderr.Wrap(err)
					}
					if targetAddr == nil {
						return nil
					}

					// send to backend target
					{
						var msg = message.ConnMessage{
							RemoteAddr: d.addr,
						}
						data, err := message.Marshal(msg)
						if err != nil {
							return stderr.Wrap(err)
						}

						n, err := conn.WriteToUDP(data, targetAddr)
						log.WithFields(logrus.Fields{
							"raddr": targetAddr,
						}).Debugf("send %d data:%v", n, msg)
						if err != nil {
							return stderr.Wrap(err)
						}
					}

					// send back to frontend
					{
						var msg = message.ConnMessage{
							RemoteAddr: targetAddr,
						}
						body, err := message.Marshal(msg)
						if err != nil {
							return stderr.Wrap(err)
						}
						n, err := conn.WriteToUDP(body, d.addr)
						log.Debugf("send %d data:%v", n, msg)
						if err != nil {
							return stderr.Wrap(err)
						}
					}
				}
				return nil
			}()
			if err != nil {
				log.Error(err)
			}
		}
	}
}
