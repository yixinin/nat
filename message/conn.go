package message

import (
	"nat/stderr"
	"net"
)

type ConnMessage struct {
	RemoteAddr *net.UDPAddr `json:"raddr"`
}

func (m *ConnMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, stderr.New(CodeInvalid, "header data size < 1")
	}
	if MessageType(data[0]) != TypeConn {
		return 0, stderr.New(CodeInvalid, "not conn msg")
	}
	return 1, nil
}
func (m *ConnMessage) SetData(data []byte) (int, error) {
	addr, err := net.ResolveUDPAddr("udp", string(data))
	if err != nil {
		return 0, err
	}
	m.RemoteAddr = addr
	return len(data), nil
}

func (m ConnMessage) GetHeader() ([]byte, error) {
	return []byte{byte(TypeConn)}, nil
}
func (m ConnMessage) GetData() ([]byte, error) {
	return []byte(m.RemoteAddr.String()), nil
}
