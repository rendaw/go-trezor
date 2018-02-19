package trezor

import "github.com/golang/protobuf/proto"

type Transport interface {
	Open() error
	Close() error
	Read() (MessageType, []byte, error)
	Write(message proto.Message) error

	// Used by Protocol
	ReadChunk() ([]byte, error)
	WriteChunk([]byte) error
}
