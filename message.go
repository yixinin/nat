package main

type StunType string

const (
	Client StunType = "Client"
	Server StunType = "Server"
)

type StunMessage struct {
	Type StunType
}
