package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"nat/stderr"
	"net"
	"runtime/debug"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
)

type BackendTunnel struct {
	rconn *net.UDPConn
	raddr *net.UDPAddr

	certFile, keyFile string

	laddr string
}

func NewBackendTunnel(localAddr string, remoteAddr *net.UDPAddr, conn *net.UDPConn, certFile, keyFile string) *BackendTunnel {
	t := &BackendTunnel{
		laddr:    localAddr,
		rconn:    conn,
		raddr:    remoteAddr,
		certFile: certFile,
		keyFile:  keyFile,
	}
	return t
}

func (t *BackendTunnel) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": t.laddr,
		"raddr": t.raddr.String(),
	})
	log.Infof("start backend tunnel")
	defer log.Infof("backend tunnel exit.")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ca, err := tls.LoadX509KeyPair(t.certFile, t.keyFile)
	if err != nil {
		return stderr.Wrap(err)
	}
	// listen quic
	lis, err := quic.Listen(t.rconn, &tls.Config{
		Certificates: []tls.Certificate{ca},
	}, &quic.Config{
		KeepAlivePeriod: 10 * time.Second,
		EnableDatagrams: true,
		Versions:        []quic.VersionNumber{quic.Version2},
		RequireAddressValidation: func(a net.Addr) bool {
			return true
		},
	})
	if err != nil {
		return stderr.Wrap(err)
	}
	log.Info("start quic accept")
	conn, err := lis.Accept(ctx)
	if err != nil {
		return stderr.Wrap(err)
	}
	log.Info("start heartbeat stream accept")
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		return stderr.Wrap(err)
	}
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()
	go func() {
		for range tk.C {
			stream.Write([]byte("::"))
		}
	}()
	log.Info("start data stream accept")
	var streamCh = make(chan quic.Stream, 1)
	defer close(streamCh)
	var acceptStream = func(conn quic.Connection) {
		for {
			stream, err := conn.AcceptStream(ctx)
			if err != nil {
				log.Error(err)
				return
			}
			streamCh <- stream
		}
	}
	go acceptStream(conn)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(streamCh)
		}()
		conn, err := lis.Accept(ctx)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"laddr": t.laddr,
			}).Errorf("accept quic conn exit with error:%v", err)
			cancel()
			return
		}

		go acceptStream(conn)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case stream, ok := <-streamCh:
			if !ok && stream == nil {
				logrus.Info("proxy msg chan closed!")
				return nil
			}
			go func() {
				err := t.handle(ctx, stream)
				if errors.Is(err, ErrorTunnelClosed) {
					logrus.Errorf("tunnel closed")
					cancel()
					return
				}
				if err != nil {
					log.Errorf("handle backend session %d error:%v", stream.StreamID(), err)
				}
			}()
		}
	}
}

func (t *BackendTunnel) handle(ctx context.Context, stream quic.Stream) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"id":    stream.StreamID(),
		"laddr": t.laddr,
		"raddr": t.raddr.String(),
	})
	log.Infof("start backend tunnel handle")
	defer log.Infof("backend tunnel hanle exit.")

	conn, err := net.Dial("tcp", t.laddr)
	if err != nil {
		return err
	}
	return Copy(ctx, conn, stream, log)
}
