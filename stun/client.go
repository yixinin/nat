package stun

import (
	"context"
	"nat/message"
	"nat/stderr"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

func handshake(ctx context.Context, conn *net.UDPConn, raddr *net.UDPAddr) error {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"raddr": raddr.String(),
	})
	log.Info("start handshake")
	defer log.Info("handshake exit.")
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tk.C:
			msg := message.HandShakeMessage{}
			data, err := message.Marshal(msg)
			if err != nil {
				return stderr.Wrap(err)
			}

			n, err := conn.WriteToUDP(data, raddr)
			log.WithFields(logrus.Fields{
				"raddr": raddr.String(),
			}).Debugf("send %d data:%v", n, msg)
			if err != nil {
				return stderr.Wrap(err)
			}
		}
	}
}
