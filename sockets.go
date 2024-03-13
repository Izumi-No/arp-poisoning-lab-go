package main

import (
	"net"

	"github.com/google/uuid"
)

type Sockets struct {
	Sockets map[uuid.UUID]net.Conn
}

func NewSockets() *Sockets {
	return &Sockets{
		Sockets: make(map[uuid.UUID]net.Conn),
	}
}

func (s *Sockets) AddSocket(name uuid.UUID, conn net.Conn) {
	s.Sockets[name] = conn
}

func (s *Sockets) RemoveSocket(name uuid.UUID) {
	delete(s.Sockets, name)
}

func (s *Sockets) GetSocket(name uuid.UUID) net.Conn {
	return s.Sockets[name]
}

func (s *Sockets) GetSockets() map[uuid.UUID]net.Conn {
	return s.Sockets
}
