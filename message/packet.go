package message

const BufferSize = 1400
const PacketSize = BufferSize - 1

type PacketMessage struct {
	Data []byte `json:"data"`
}

func NewPacketMessage(data []byte) []PacketMessage {
	if len(data) <= PacketSize {
		return []PacketMessage{{Data: data}}
	}
	ps := make([]PacketMessage, 0, len(data)/PacketSize+1)
	for i := 0; i < len(data); i += PacketSize {
		var end = i + PacketSize
		if end > len(data) {
			end = len(data)
		}
		ps = append(ps, PacketMessage{Data: data[i:end]})
	}
	return ps
}

func (m *PacketMessage) SetData(data []byte) (int, error) {
	m.Data = data
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
	return m.Data, nil
}
