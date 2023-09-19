package message

import (
	"encoding/binary"
	"nat/stderr"
)

type TunnelMessage struct {
	Id uint64
}

func NewTunnelMessage(id uint64) TunnelMessage {
	return TunnelMessage{
		Id: id,
	}
}

func (TunnelMessage) Type() MessageType {
	return TypeTunnel
}

func (m *TunnelMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, stderr.New(CodeInvalid, "header data size < 1")
	}
	if MessageType(data[0]) != TypeTunnel {
		return 0, stderr.New(CodeInvalid, "not tunnel msg")
	}
	return 1, nil
}
func (m *TunnelMessage) SetData(data []byte) (int, error) {
	m.Id = binary.BigEndian.Uint64(data)
	return len(data), nil
}

func (m TunnelMessage) GetHeader() ([]byte, error) {
	return []byte{byte(TypeTunnel)}, nil
}
func (m TunnelMessage) GetData() ([]byte, error) {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, m.Id)
	return data, nil
}
