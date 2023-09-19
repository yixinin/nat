package message

type HandShakeMessage struct {
}

func (m *HandShakeMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, ErrorInvalidMessage
	}
	if MessageType(data[0]) != TypeHandShake {
		return 0, ErrorInvalidMessage
	}
	return 1, nil
}

func (m HandShakeMessage) GetHeader() ([]byte, error) {
	return []byte{byte(TypeHandShake)}, nil
}
