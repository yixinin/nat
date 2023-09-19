package message

import (
	"fmt"
	"nat/stderr"
)

type ClientType byte

const (
	Backend  = 1
	Frontend = 2
)

func (t ClientType) IsValid() bool {
	if t != Backend && t != Frontend {
		return false
	}
	return true
}

type StunMessage struct {
	ClientType ClientType `json:"-"`
	FQDN       string     `json:"fqnd"`
}

func NewStunMessage(t ClientType, fqdn string) StunMessage {
	return StunMessage{
		ClientType: t,
		FQDN:       fqdn,
	}
}

func (m *StunMessage) SetData(data []byte) (int, error) {
	m.FQDN = string(data)
	return len(data), nil
}

func (m *StunMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 2 {
		return 0, stderr.New(CodeInvalid, "header size < 2")
	}
	if MessageType(data[0]) != TypeStun {
		return 0, stderr.New(CodeInvalid, "not stun msg")
	}
	m.ClientType = ClientType(data[1])
	if !m.ClientType.IsValid() {
		return 0, stderr.New(CodeInvalid, fmt.Sprintf("client type:%d invalid", m.ClientType))
	}
	return 2, nil
}

func (m StunMessage) GetHeader() ([]byte, error) {
	if !m.ClientType.IsValid() {
		return nil, stderr.New(CodeInvalid, fmt.Sprintf("client type:%d invalid", m.ClientType))
	}
	return []byte{byte(TypeStun), byte(m.ClientType)}, nil
}
func (m StunMessage) GetData() ([]byte, error) {
	return []byte(m.FQDN), nil
}
