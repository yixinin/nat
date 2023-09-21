package tunnel

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"runtime/debug"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
)

type BackendTunnel struct {
	rconn *net.UDPConn
	raddr *net.UDPAddr

	localAddr string
}

func NewBackendTunnel(localAddr string, remoteAddr *net.UDPAddr, conn *net.UDPConn) *BackendTunnel {
	t := &BackendTunnel{
		localAddr: localAddr,
		rconn:     conn,
		raddr:     remoteAddr,
	}
	return t
}

func (t *BackendTunnel) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": t.localAddr,
		"raddr": t.raddr.String(),
	})
	log.Infof("start backend tunnel")
	defer log.Infof("backend tunnel exit.")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lis, err := quic.Listen(t.rconn, &tls.Config{}, &quic.Config{})
	if err != nil {
		return err
	}
	var steamCh = make(chan quic.Stream, 1)
	defer close(steamCh)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("stacks", string(debug.Stack())).Errorf("recovered:%v", r)
			}
			close(steamCh)
		}()
		conn, err := lis.Accept(ctx)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"laddr": t.localAddr,
			}).Errorf("accept quic conn exit with error:%v", err)
			cancel()
			return
		}

		for {
			stream, err := conn.AcceptStream(ctx)
			if err != nil {
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"laddr": t.localAddr,
					}).Errorf("accept exit with error:%v", err)
					cancel()
					return
				}
				steamCh <- stream
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case stream, ok := <-steamCh:
			if !ok && stream == nil {
				logrus.Info("proxy msg chan closed!")
				return nil
			}
			go func() {
				err := t.handle(ctx, stream)
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
		"laddr": t.localAddr,
		"raddr": t.raddr.String(),
	})
	log.Infof("start backend tunnel handle")
	defer log.Infof("backend tunnel hanle exit.")

	conn, err := net.Dial("tcp", t.localAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(conn, stream)
		if err != nil {
			log.Errorf("local copy to proxy error:%v", err)
		}

	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(stream, conn)
		if err != nil {
			log.Errorf("proxy copy to local error:%v", err)
		}
	}()
	wg.Wait()
	return ctx.Err()
}
