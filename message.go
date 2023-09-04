package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

type MType uint8

const (
	TypeSdp  MType = 1
	TypeEcho MType = 2
)

func (t MType) String() string {
	switch t {
	case TypeSdp:
		return "sdp"
	case TypeEcho:
		return "echo"
	}
	return strconv.Itoa(int(t))
}

type Message struct {
	Type     MType
	ClientId uint64
	Payload  []byte
}

func NewMessage(cid uint64, t MType, data []byte) Message {
	return Message{
		ClientId: cid,
		Type:     t,
		Payload:  data,
	}
}

func Encode(m Message) []byte {
	var data = make([]byte, 1+8+len(m.Payload))
	data[0] = byte(m.Type)
	binary.BigEndian.PutUint64(data[1:1+8], m.ClientId)
	copy(data[1+8:], m.Payload)
	return data
}

func Decode(data []byte) (Message, error) {
	msg := Message{}
	if len(data) == 0 {
		return msg, errors.New("invalid data")
	}
	msg.Type = MType(data[0])
	switch msg.Type {
	case TypeEcho:
		return msg, nil
	case TypeSdp:
		if len(data) < 1+8 {
			return msg, errors.New("invalid sdp data")
		}
	}
	msg.ClientId = binary.BigEndian.Uint64(data[1 : 1+8])
	msg.Payload = data[1+8:]
	return msg, nil
}

func (m Message) String() string {
	return fmt.Sprintf("%s %d %s", m.Type, m.ClientId, m.Payload)
}
