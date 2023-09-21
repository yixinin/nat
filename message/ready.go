package message

import "nat/stderr"

type ReadyMessage struct {
}

func (ReadyMessage) Type() MessageType {
	return TypeReady
}

func (m *ReadyMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, stderr.New(CodeInvalid, "header size < 2")
	}
	if MessageType(data[0]) != TypeReady {
		return 0, stderr.New(CodeInvalid, "not hb msg")
	}
	return 1, nil
}
func (m *ReadyMessage) SetData(data []byte) (int, error) {
	return 0, nil
}

func (m ReadyMessage) GetHeader() ([]byte, error) {
	return []byte{byte(TypeReady)}, nil
}
func (m ReadyMessage) GetData() ([]byte, error) {
	return nil, nil
}
