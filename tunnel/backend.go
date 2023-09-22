package tunnel

import (
	"context"
	"crypto/tls"
	"io"
	"nat/message"
	"net"
	"runtime/debug"
	"sync"
	"time"

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
			log.Info(string(buf[:n]))
			return err
		}
		if msg.Type() == message.TypeReady {
			break
		}
	}
	log.Info("ready quic accept")

	ca, err := tls.LoadX509KeyPair("quic.crt", "quic.key")
	if err != nil {
		return err
	}
	// listen quic
	lis, err := quic.Listen(t.rconn, &tls.Config{
		Certificates: []tls.Certificate{ca},
	}, &quic.Config{})
	if err != nil {
		return err
	}
	conn, err := lis.Accept(ctx)
	if err != nil {
		return err
	}

	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		return err
	}
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()
	go func() {
		for range tk.C {
			stream.Write([]byte("::"))
		}
	}()

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
				"laddr": t.localAddr,
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
			log.Errorf("stream copy to conn error:%v", err)
		} else {
			log.Info("end stream copy to conn")
		}

	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(stream, conn)
		if err != nil {
			log.Errorf("conn copy to stream error:%v", err)
		} else {
			log.Info("end conn copy to stream")
		}
	}()
	wg.Wait()
	return ctx.Err()
}
