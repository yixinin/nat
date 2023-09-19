package message

import "nat/stderr"

type HeartbeatMessage struct {
	NoRelay bool `json:"-"`
}

func (m *HeartbeatMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 2 {
		return 0, stderr.New(CodeInvalid, "header size < 2")
	}
	if MessageType(data[0]) != TypeHeartbeat {
		return 0, stderr.New(CodeInvalid, "not hb msg")
	}
	m.NoRelay = data[1] == 1
	return 2, nil
}
func (m *HeartbeatMessage) SetData(data []byte) (int, error) {
	return 0, nil
}

func (m HeartbeatMessage) GetHeader() ([]byte, error) {
	var b1 byte = 0
	if m.NoRelay {
		b1 = 1
	}
	return []byte{byte(TypeHeartbeat), b1}, nil
}
func (m HeartbeatMessage) GetData() ([]byte, error) {
	return nil, nil
}
