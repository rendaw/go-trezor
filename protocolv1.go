package trezor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const REPLEN_V1 = 64

type ProtocolV1 struct {
}

func ProtocolV1New() (Protocol, error) {
	return ProtocolV1{}, nil
}

func (self ProtocolV1) sessionBegin(transport Transport) error {
	return nil
}

func (self ProtocolV1) sessionEnd(transport Transport) error {
	return nil
}

func (self ProtocolV1) write(transport Transport, messageType int32, data []byte) error {
	dataHeader := [2 + 2 + 4]byte{}
	dataHeader[0] = '#'
	dataHeader[1] = '#'
	binary.BigEndian.PutUint16(dataHeader[2:4], uint16(messageType))
	binary.BigEndian.PutUint32(dataHeader[4:8], uint32(len(data)))
	data = append(dataHeader[:], data...)
	for len(data) > 0 {
		chunk := [REPLEN_V1]byte{}
		chunk[0] = '?'
		off := copy(chunk[1:], data)
		data = data[off:]
		err := transport.writeChunk(chunk[:])
		if err != nil {
			return err
		}
	}
	return nil
}

func (self ProtocolV1) read(transport Transport) (int32, []byte, error) {
	chunk, err := transport.readChunk()
	if err != nil {
		return 0, nil, err
	}
	messageType, dataLen, data, err := parseFirst(self, chunk)
	if err != nil {
		return 0, nil, err
	}
	for uint32(len(data)) < dataLen {
		chunk, err := transport.readChunk()
		if err != nil {
			return 0, nil, err
		}
		data = append(data, chunk...)
	}
	return messageType, data[:dataLen], nil
}

func parseFirst(proto ProtocolV1, chunk []byte) (int32, uint32, []byte, error) {
	magic := []byte{'?', '#', '#'}
	if !bytes.Equal(chunk[:3], magic) {
		return 0, 0, nil, fmt.Errorf("Expected magic characters %s, got %s", hex.EncodeToString(magic), hex.EncodeToString(chunk[0:3]))
	}
	offset := 3
	messageType := int32(binary.BigEndian.Uint16(chunk[offset : offset+2]))
	offset += 2
	dataLen := binary.BigEndian.Uint32(chunk[offset : offset+4])
	offset += 4
	return messageType, dataLen, chunk[offset:], nil
}
