package trezor

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const REPLEN_V2 = 64

type ProtocolV2 struct {
	hasSession bool
	session    uint32
}

func ProtocolV2New() (Protocol, error) {
	return ProtocolV2{
		hasSession: false,
		session:    0,
	}, nil
}

func (self ProtocolV2) sessionBegin(transport Transport) error {
	chunk := [REPLEN_V2]byte{}
	chunk[0] = 3
	for i := 1; i < len(chunk); i++ {
		chunk[i] = 0
	}
	err := transport.writeChunk(chunk[:])
	if err != nil {
		return err
	}
	resp, err := transport.readChunk()
	if err != nil {
		return err
	}
	err = self.parseSessionOpen(resp)
	if err != nil {
		return err
	}
	return nil
}

func (self ProtocolV2) sessionEnd(transport Transport) error {
	if !self.hasSession {
		return nil
	}
	chunk := [REPLEN_V2]byte{}
	chunk[0] = 0x04
	binary.BigEndian.PutUint32(chunk[1:5], self.session)
	for i := 5; i < len(chunk); i++ {
		chunk[i] = 0x00
	}
	err := transport.writeChunk(chunk[:])
	if err != nil {
		return err
	}
	resp, err := transport.readChunk()
	if err != nil {
		return err
	}
	magic := resp[0]
	if magic != 0x04 {
		return fmt.Errorf("Expected session close (0x04), got %d", hex.EncodeToString([]byte{magic}))
	}
	self.hasSession = false
	return nil
}

func (self ProtocolV2) write(transport Transport, messageType int32, data []byte) error {
	if !self.hasSession {
		return fmt.Errorf("Missing session for v2 protocol")
	}
	dataHeader := [8]byte{}
	binary.BigEndian.PutUint32(dataHeader[0:4], uint32(messageType))
	binary.BigEndian.PutUint32(dataHeader[4:8], uint32(len(data)))
	data = append(dataHeader[:], data...)
	var seq uint32 = -1
	for len(data) > 0 {
		var repHeader []byte
		if seq < 0 {
			repHeader = [5]byte{}[:]
			repHeader[0] = 0x01
			binary.BigEndian.PutUint32(repHeader[1:5], self.session)
		} else {
			repHeader = [9]byte{}[:]
			repHeader[0] = 0x02
			binary.BigEndian.PutUint32(repHeader[1:5], self.session)
			binary.BigEndian.PutUint32(repHeader[1:5], seq)
		}
		chunk := [REPLEN_V2]byte{}
		dataLen := REPLEN_V2 - len(repHeader)
		off := copy(chunk[0:], repHeader)
		off = copy(chunk[off:], data)
		for i := off; i < len(chunk); i++ {
			chunk[i] = 0
		}
		err := transport.writeChunk(chunk[:])
		if err != nil {
			return err
		}
		data = data[dataLen:]
		seq += 1
	}
	return nil
}

func (self ProtocolV2) read(transport Transport) (int32, []byte, error) {
	if !self.hasSession {
		return 0, nil, fmt.Errorf("Missing session for v2 protocol")
	}

	chunk, err := transport.readChunk()
	if err != nil {
		return 0, nil, err
	}
	messageType, dataLen, data, err := parseFirstV2(self, chunk)
	if err != nil {
		return 0, nil, err
	}

	for uint32(len(data)) < dataLen {
		chunk, err := transport.readChunk()
		if err != nil {
			return 0, nil, err
		}
		nextData, err := parseNextV2(self, chunk)
		if err != nil {
			return 0, nil, err
		}
		data = append(data, nextData...)
	}

	return messageType, data[:dataLen], nil
}

func parseFirstV2(proto ProtocolV2, chunk []byte) (int32, uint32, []byte, error) {
	offset := 0
	magic := chunk[offset]
	offset += 1
	if magic != 0x01 {
		return 0, 0, nil, fmt.Errorf("Expected magic character 0x01, got %s", hex.EncodeToString([]byte{magic}))
	}
	sessionBytes := chunk[offset : offset+4]
	session := binary.BigEndian.Uint32(sessionBytes)
	offset += 4
	if session != proto.session {
		protoSessionBytes := [4]byte{}
		binary.BigEndian.PutUint32(protoSessionBytes[:], proto.session)
		return 0, 0, nil, fmt.Errorf("Session mismatch, expected %s, got %s", hex.EncodeToString(protoSessionBytes[:]), hex.EncodeToString(sessionBytes))
	}
	messageType := int32(binary.BigEndian.Uint32(chunk[offset : offset+4]))
	offset += 4
	dataLen := binary.BigEndian.Uint32(chunk[offset : offset+4])
	offset += 4
	return messageType, dataLen, chunk[offset:], nil
}

func parseNextV2(proto ProtocolV2, chunk []byte) ([]byte, error) {
	offset := 0
	magic := chunk[offset]
	offset += 1
	if magic != 0x02 {
		return nil, fmt.Errorf("Expected magic character 0x02, got %s", hex.EncodeToString([]byte{magic}))
	}
	sessionBytes := chunk[offset : offset+4]
	session := binary.BigEndian.Uint32(sessionBytes)
	offset += 4
	if session != proto.session {
		protoSessionBytes := [4]byte{}
		binary.BigEndian.PutUint32(protoSessionBytes[:], proto.session)
		return nil, fmt.Errorf("Session mismatch, expected %s, got %s", hex.EncodeToString(protoSessionBytes[:]), hex.EncodeToString(sessionBytes))
	}
	offset += 4 // skip sequence
	return chunk[offset:], nil
}

func (self ProtocolV2) parseSessionOpen(resp []byte) error {
	magic := resp[0]
	session := binary.BigEndian.Uint32(resp[1:5])
	if magic != 0x03 {
		return fmt.Errorf("Expected magic character 0x03, got %s", hex.EncodeToString([]byte{magic}))
	}
	self.session = session
	self.hasSession = true
	return nil
}
