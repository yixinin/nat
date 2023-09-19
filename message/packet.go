package message

type PacketMessage struct {
	Data []byte
}

func NewPacketMessage(data []byte) PacketMessage {
	return PacketMessage{Data: data}
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
