package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"nat/stderr"
	"net"
	"os"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
)

type Lconn struct {
	Conn      net.Conn
	data      []byte
	FQDN      string
	httpsAddr string
}

func NewLconn(conn net.Conn, data []byte, fqdn, addr string) Lconn {
	return Lconn{
		Conn:      conn,
		data:      data,
		FQDN:      fqdn,
		httpsAddr: addr,
	}
}

type FrontendTunnel struct {
	rconn *net.UDPConn
	raddr *net.UDPAddr

	FQDN string

	connCh chan Lconn
}

func NewFrontendTunnel(fqdn string, remoteAddr *net.UDPAddr, conn *net.UDPConn) *FrontendTunnel {
	t := &FrontendTunnel{
		rconn:  conn,
		raddr:  remoteAddr,
		FQDN:   fqdn,
		connCh: make(chan Lconn, 1),
	}
	return t
}
func (t *FrontendTunnel) Accept(lconn Lconn) {
	t.connCh <- lconn
}

func (t *FrontendTunnel) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"raddr": t.raddr.String(),
	})
	log.Infof("start frontend tunnel")
	defer log.Infof("frontend tunnel exit.")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	qctx, qcancel := context.WithTimeout(ctx, 30*time.Second)
	defer qcancel()

	log.Infof("dial quic raddr:%s", t.raddr)

	quicConn, err := quic.Dial(qctx, t.rconn, t.raddr, &tls.Config{InsecureSkipVerify: true}, &quic.Config{
		EnableDatagrams: true,
		Versions:        []quic.VersionNumber{quic.Version2},
		RequireAddressValidation: func(a net.Addr) bool {
			return true
		},
	})
	if err != nil {
		return stderr.Wrap(err)
	}

	hbStream, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		return stderr.Wrap(err)
	}

	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()
	go func() {
		for range tk.C {
			hbStream.Write([]byte("::"))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case lconn, ok := <-t.connCh:
			if !ok && lconn.Conn == nil {
				log.Debug("accept chan closed!")
				return nil
			}

			sctx, scancel := context.WithTimeout(ctx, 10*time.Second)
			defer scancel()
			stream, err := quicConn.OpenStreamSync(sctx)
			if err != nil {
				if os.IsTimeout(err) {
					if err.Error() == NoRecentNetworkActivity {
						return stderr.New("closed", NoRecentNetworkActivity)
					}
				}
				log.Errorf("open stream error:%v", err)
				continue
			}
			go func() {
				if len(lconn.httpsAddr) > 0 {
					_, err := fmt.Fprint(lconn.Conn, "HTTP/1.1 200 Connection established\r\n\r\n")
					if err != nil {
						log.Errorf("write back https connected error:%v", err)
						return
					}
				} else {
					_, err := stream.Write(lconn.data)
					if err != nil {
						log.Errorf("write header data error:%v", err)
						return
					}
				}

				if err := t.handle(ctx, lconn.Conn, stream); err != nil {
					log.WithField("id", stream.StreamID()).Errorf("handle frontend session error:%v", err)
				}
			}()

		}
	}
}

func (t *FrontendTunnel) handle(ctx context.Context, conn net.Conn, stream quic.Stream) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"id": stream.StreamID(),
	})
	log.Debug("start handle")
	defer log.Debug("handle exit.")
	return Copy(ctx, conn, stream, log)
}
