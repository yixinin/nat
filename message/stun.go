package message

import "nat/stderr"

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

func (m *StunMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 2 {
		return 0, ErrorInvalidMessage
	}
	if MessageType(data[0]) != TypeStun {
		return 0, ErrorInvalidMessage
	}
	m.ClientType = ClientType(data[1])
	if !m.ClientType.IsValid() {
		return 0, ErrorInvalidMessage
	}
	return 2, nil
}

func (m StunMessage) GetHeader() ([]byte, error) {
	if !m.ClientType.IsValid() {
		return nil, stderr.Wrap(ErrorInvalidMessage)
	}
	return []byte{byte(TypeStun), byte(m.ClientType)}, nil
}
func (m StunMessage) GetData() ([]byte, error) {
	return nil, nil
}
