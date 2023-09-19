package message

import (
	"encoding/binary"
	"sync/atomic"
)

const BufferSize = 1400
const PacketSize = BufferSize - 1

var id = atomic.Uint64{}

type PacketMessage struct {
	Id   uint64 `json:"id"`
	Data []byte `json:"data"`
}

func NewPacketMessage(data []byte) []PacketMessage {
	if len(data) <= PacketSize {
		return []PacketMessage{{
			Id:   id.Add(1),
			Data: data,
		}}
	}
	ps := make([]PacketMessage, 0, len(data)/PacketSize+1)
	for i := 0; i < len(data); i += PacketSize {
		var end = i + PacketSize
		if end > len(data) {
			end = len(data)
		}
		ps = append(ps, PacketMessage{
			Id:   id.Add(1),
			Data: data[i:end],
		})
	}
	return ps
}

func (m *PacketMessage) SetData(data []byte) (int, error) {
	if len(data) < 8 {
		return 0, ErrorInvalidMessage
	}
	m.Id = binary.BigEndian.Uint64(data[:8])
	m.Data = data[8:]
	return len(data), nil
}

func (m *PacketMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, ErrorInvalidMessage
	}
	if MessageType(data[0]) != TypePacket {
		return 0, ErrorInvalidMessage
	}
	return 1, nil
}

func (m PacketMessage) GetHeader() ([]byte, error) {
	return []byte{byte(TypePacket)}, nil
}
func (m PacketMessage) GetData() ([]byte, error) {
	data := make([]byte, 8+len(m.Data))
	binary.BigEndian.PutUint64(data[:8], m.Id)
	copy(data[8:], m.Data)
	return data, nil
}
