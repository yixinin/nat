package message

import "net"

type ConnMessage struct {
	RemoteAddr *net.UDPAddr `json:"raddr"`
}

func (m *ConnMessage) SetHeader(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, ErrorInvalidMessage
	}
	if MessageType(data[0]) != TypeConn {
		return 0, ErrorInvalidMessage
	}
	return 1, nil
}

func (m ConnMessage) GetHeader() ([]byte, error) {
	return []byte{byte(TypeConn)}, nil
}
func (m ConnMessage) GetData() ([]byte, error) {
	return nil, nil
}
