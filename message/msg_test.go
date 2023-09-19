package message_test

import (
	"fmt"
	"nat/message"
	"testing"
)

func TestHb(t *testing.T) {
	msg := message.HeartbeatMessage{
		NoRelay: true,
	}
	data, err := message.Marshal(msg)
	fmt.Println(data, err)
}

func TestHs(t *testing.T) {
	msg := message.HandShakeMessage{}
	data, err := message.Marshal(msg)
	fmt.Println(data, err)
}
