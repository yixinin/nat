package message

import (
	"encoding/json"
	"errors"
)

var ErrorInvalidMessage = errors.New("invalid message")

type MessageType byte

const (
	TypeStun      MessageType = 1
	TypeConn      MessageType = 2
	TypeHandShake MessageType = 3
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
}

type MessageMarshal interface {
	GetHeader() ([]byte, error)
}

func Unmarshal(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, ErrorInvalidMessage
	}
	var msg MessageUnmarshal
	switch MessageType(data[0]) {
	case TypeStun:
		msg = &StunMessage{}
	case TypeConn:
		msg = &ConnMessage{}
	case TypeHandShake:
		msg = &HandShakeMessage{}
	default:
		return nil, ErrorInvalidMessage
	}

	n, err := msg.SetHeader(data)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data[n:], msg)
	return msg, err
}

func Marshal(msg MessageMarshal) ([]byte, error) {
	var header, err = msg.GetHeader()
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var data = make([]byte, len(body)+len(header))
	copy(data, header)
	copy(data[len(header):], body)
	return data, nil
}
