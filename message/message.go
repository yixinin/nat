package message

import (
	"errors"
	"fmt"
	"nat/stderr"
)

var ErrorInvalidMessage = errors.New("invalid message")

const CodeInvalid = "invalid"

type MessageType byte

const (
	TypeStun      MessageType = 1
	TypeConn      MessageType = 2
	TypeHandShake MessageType = 3
	TypeHeartbeat MessageType = 4
	TypePacket    MessageType = 5
)

func (t MessageType) IsValid() bool {
	if t != TypeConn && t != TypeStun {
		return false
	}
	return true
}

type Message interface {
	MessageMarshal
	MessageUnmarshal
}

type MessageUnmarshal interface {
	SetHeader(header []byte) (int, error)
	SetData(data []byte) (int, error)
}

type MessageMarshal interface {
	GetHeader() ([]byte, error)
	GetData() ([]byte, error)
}

func Unmarshal(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, stderr.Wrap(fmt.Errorf("empty data"))
	}
	var msg MessageUnmarshal
	switch t := MessageType(data[0]); t {
	case TypeStun:
		msg = &StunMessage{}
	case TypeConn:
		msg = &ConnMessage{}
	case TypeHandShake:
		msg = &HandShakeMessage{}
	case TypeHeartbeat:
		msg = &HeartbeatMessage{}
	case TypePacket:
		msg = &PacketMessage{}
	default:
		return nil, stderr.Wrap(fmt.Errorf("unknown msg type:%d", t))
	}

	n, err := msg.SetHeader(data)
	if err != nil {
		return nil, stderr.Wrap(err)
	}
	_, err = msg.SetData(data[n:])
	return msg, err
}

func Marshal(msg MessageMarshal) ([]byte, error) {
	var header, err = msg.GetHeader()
	if err != nil {
		return nil, stderr.Wrap(err)
	}
	body, err := msg.GetData()
	if err != nil {
		return nil, stderr.Wrap(err)
	}
	var data = make([]byte, len(body)+len(header))
	copy(data, header)
	copy(data[len(header):], body)
	return data, nil
}
