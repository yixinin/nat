package message

import (
	"encoding/binary"
	"nat/stderr"
)

const BufferSize = 1400
const PacketSize = BufferSize - 1 - 8

type PacketMessage struct {
	Id   uint64 `json:"id"`
	Seq  uint64
	Data []byte `json:"data"`
}

func NewPacketMessage(id, seq uint64, data []byte) []PacketMessage {
	if len(data) <= PacketSize {
		return []PacketMessage{{
			Id:   id,
			Seq:  seq,
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
			Id:   id,
			Seq:  seq,
			Data: data[i:end],
		})
		seq++
	}
	return ps
}

func (PacketMessage) Type() MessageType {
	return TypePacket
}

func (m *PacketMessage) SetData(data []byte) (int, error) {
	if len(data) < 16 {
		return 0, stderr.New(CodeInvalid, "data size < 16")
	}
	m.Id = binary.BigEndian.Uint64(data[:8])
	m.Seq = binary.BigEndian.Uint64(data[8:16])
	m.Data = data[16:]
	return len(data), nil
}

func (m *PacketMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, stderr.New(CodeInvalid, "header size < 1")
	}
	if MessageType(data[0]) != TypePacket {
		return 0, stderr.New(CodeInvalid, "not packet msg")
	}
	return 1, nil
}

func (m PacketMessage) GetHeader() ([]byte, error) {
	return []byte{byte(TypePacket)}, nil
}
func (m PacketMessage) GetData() ([]byte, error) {
	data := make([]byte, 16+len(m.Data))
	binary.BigEndian.PutUint64(data[:8], m.Id)
	binary.BigEndian.PutUint64(data[8:16], m.Seq)
	copy(data[16:], m.Data)
	return data, nil
}
