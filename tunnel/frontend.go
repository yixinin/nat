package tunnel

import (
	"context"
	"crypto/tls"
	"io"
	"nat/message"
	"nat/stderr"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
)

type FrontendTunnel struct {
	rconn *net.UDPConn
	raddr *net.UDPAddr

	localAddr string
}

func NewFrontendTunnel(localAddr string, remoteAddr *net.UDPAddr, conn *net.UDPConn) *FrontendTunnel {
	t := &FrontendTunnel{
		localAddr: localAddr,
		rconn:     conn,
		raddr:     remoteAddr,
	}
	return t
}

func (t *FrontendTunnel) Run(ctx context.Context) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"laddr": t.localAddr,
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
			break
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	qctx, qcancel := context.WithTimeout(ctx, 10*time.Second)
	defer qcancel()

	quicConn, err := quic.Dial(qctx, t.rconn, t.raddr, &tls.Config{InsecureSkipVerify: true}, &quic.Config{})
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", t.localAddr)
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
					"laddr": t.localAddr,
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

<<<<<<< HEAD
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
=======
			msg := message.NewTunnelMessage(sessid.Add(1))
			t.Proxy.SendCmdMessage(&msg)

			go func(msg message.TunnelMessage) {
				if err := t.handle(ctx, sessid.Load(), conn); err != nil {
					log.WithField("id", sessid.Load()).Errorf("handle frontend session error:%v", err)
>>>>>>> 5b4c725f6cecd96fd26f7ea81819e44c16109d4d
				}
			}()

		}
	}
}

func (t *FrontendTunnel) handle(ctx context.Context, conn net.Conn, stream quic.Stream) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"id":    stream.StreamID(),
		"laddr": t.localAddr,
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
			log.Errorf("local copy to proxy error:%v", err)
		}
		log.Debugf("end local copy %d to proxy", n)
	}()
	go func() {
		defer wg.Done()
		n, err := io.Copy(stream, conn)
		if err != nil {
			log.Errorf("proxy copy to local error:%v", err)
		}
		log.Debugf("end proxy copy %d to local", n)
	}()
	wg.Wait()
	return ctx.Err()
}
