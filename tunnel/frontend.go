package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"nat/message"
	"nat/stderr"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
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

	// send peer ready
	data, _ := message.Marshal(message.ReadyMessage{})
	_, err := t.rconn.WriteToUDP(data, t.raddr)
	if err != nil {
		return err
	}
	// wait peer ready
	var buf = make([]byte, 32)
	for {
		n, _, err := t.rconn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		msg, err := message.Unmarshal(buf[:n])
		if err != nil {
			return err
		}
		if msg.Type() == message.TypeReady {
			log.Info("recv peer ready message, start quic dial")
			break
		}
	}

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
	defer conn.Close()
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, err := io.Copy(conn, stream)
		if err != nil {
			log.Errorf("stream copy to conn error:%v", err)
		} else {
			log.Debugf("end stream copy %d to conn", n)
		}
	}()
	go func() {
		defer wg.Done()
		n, err := io.Copy(stream, conn)
		if err != nil {
			log.Errorf("conn copy to stream error:%v", err)
		} else {
			log.Debugf("end conn copy %d to stream", n)
		}

	}()
	wg.Wait()
	return ctx.Err()
}
