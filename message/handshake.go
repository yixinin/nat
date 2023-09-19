package message

import "nat/stderr"

type HandShakeMessage struct {
	NoRelay bool `json:"-"`
}

func (m *HandShakeMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, stderr.New(CodeInvalid, "header size < 1")
	}
	if MessageType(data[0]) != TypeHandShake {
		return 0, stderr.New(CodeInvalid, "not handshake msg")
	}
	m.NoRelay = data[1] == 1
	return 2, nil
}
func (m *HandShakeMessage) SetData(data []byte) (int, error) {
	return 0, nil
}

func (m HandShakeMessage) GetHeader() ([]byte, error) {
	var b1 byte = 0
	if m.NoRelay {
		b1 = 1
	}
	return []byte{byte(TypeHandShake), b1}, nil
}

func (m HandShakeMessage) GetData() ([]byte, error) {
	return nil, nil
}
