package message

import (
	"errors"
	"nat/stderr"
)

var ErrorInvalidMessage = errors.New("invalid message")

type MessageType byte

const (
	TypeStun      MessageType = 1
	TypeConn      MessageType = 2
	TypeHandShake MessageType = 3
	TypeHeartbeat MessageType = 4
	TypePacket    MessageType = 4
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
		return nil, stderr.Wrap(ErrorInvalidMessage)
	}
	var msg MessageUnmarshal
	switch MessageType(data[0]) {
	case TypeStun:
		msg = &StunMessage{}
	case TypeConn:
		msg = &ConnMessage{}
	case TypeHandShake:
		msg = &HandShakeMessage{}
	case TypePacket:
		msg = &PacketMessage{}
	default:
		return nil, stderr.Wrap(ErrorInvalidMessage)
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
