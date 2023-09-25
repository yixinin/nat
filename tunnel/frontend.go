package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"nat/stderr"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
)

type FrontendTunnel struct {
	rconn *net.UDPConn
	raddr *net.UDPAddr

	FQDN  string
	laddr string
	port  uint16

	proxy bool
	hosts bool
}

func NewFrontendTunnel(fqdn, localAddr string, remoteAddr *net.UDPAddr, conn *net.UDPConn) *FrontendTunnel {
	ss := strings.Split(localAddr, ":")
	var port uint16
	if len(ss) == 2 {
		p, err := strconv.ParseUint(ss[1], 10, 16)
		if err != nil {
			logrus.Errorf("parse local port error:%v", err)
		}
		port = uint16(p)
	}
	t := &FrontendTunnel{
		laddr: localAddr,
		rconn: conn,
		raddr: remoteAddr,
		FQDN:  fqdn,
		port:  port,
	}
	return t
}

func (t *FrontendTunnel) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": t.laddr,
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
	if t.hosts {
		ok := setHosts(t.FQDN)
		defer cleanHosts(t.FQDN)
		if ok {
			var addr string
			switch t.port {
			case 80:
				addr = fmt.Sprintf("http://%s", t.FQDN)
			case 443:
				addr = fmt.Sprintf("https://%s", t.FQDN)
			default:
				addr = fmt.Sprintf("http://%s:%d", t.FQDN, t.port)
			}
			logrus.Infof("write hosts success, now you can visit site %s", addr)
		} else {
			var addr string
			switch t.port {
			case 80:
				addr = fmt.Sprintf("http://%s", "localhost")
			case 443:
				addr = fmt.Sprintf("https://%s", "localhost")
			default:
				addr = fmt.Sprintf("http://%s:%d", "localhost", t.port)
			}
			logrus.Infof("write hosts fail, you can still visit site %s by localhost:%s", t.FQDN, addr)
		}
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

	lis, err := net.Listen("tcp", t.laddr)
	if err != nil {
		return stderr.Wrap(err)
	}
	defer lis.Close()

	var connCh = make(chan net.Conn, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(connCh)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := lis.Accept()
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"laddr": t.laddr,
				}).Errorf("accept exit with error:%v", err)
				cancel()
				return
			}
			connCh <- conn
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case conn, ok := <-connCh:
			if !ok && conn == nil {
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
				if err := t.handle(ctx, conn, stream); err != nil {
					log.WithField("id", stream.StreamID()).Errorf("handle frontend session error:%v", err)
				}
			}()

		}
	}
}

func (t *FrontendTunnel) handle(ctx context.Context, conn net.Conn, stream quic.Stream) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"id":    stream.StreamID(),
		"laddr": t.laddr,
	})
	log.Debug("start handle")
	defer log.Debug("handle exit.")
	return Copy(ctx, conn, stream, log)
}
