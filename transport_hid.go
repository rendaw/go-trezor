package trezor

import (
	"fmt"
	"os"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/karalabe/hid"
)

func devTrezor1() (uint16, uint16)   { return 0x534c, 0x0001 }
func devTrezor2() (uint16, uint16)   { return 0x1209, 0x53c1 }
func devTrezor2BL() (uint16, uint16) { return 0x1209, 0x53c0 }

func isWirelink(dev hid.DeviceInfo) bool {
	return dev.UsagePage == 0xFF00 || dev.Interface == 0
}

func isDebuglink(dev hid.DeviceInfo) bool {
	return dev.UsagePage == 0xFF01 || dev.Interface == 1
}

func isTrezor1(dev hid.DeviceInfo) bool {
	gotVendor, gotProduct := devTrezor1()
	return dev.VendorID == gotVendor && dev.ProductID == gotProduct
}
func isTrezor2(dev hid.DeviceInfo) bool {
	gotVendor, gotProduct := devTrezor2()
	return dev.VendorID == gotVendor && dev.ProductID == gotProduct
}
func isTrezor2BL(dev hid.DeviceInfo) bool {
	gotVendor, gotProduct := devTrezor2BL()
	return dev.VendorID == gotVendor && dev.ProductID == gotProduct
}

func enumerate() ([]Transport, error) {
	out := []Transport{}
	for _, devList := range [][]hid.DeviceInfo{
		hid.Enumerate(devTrezor1()),
		hid.Enumerate(devTrezor2()),
		hid.Enumerate(devTrezor2BL()),
	} {
		for _, dev := range devList {
			if isWirelink(dev) {
				continue
			}
			transport, err := HidTransportNew(dev)
			if err != nil {
				return nil, err
			}
			out = append(out, transport)
		}
	}
	return out, nil
}

type HidHandle struct {
	count  int
	handle *hid.Device
}

func (self HidHandle) open(info hid.DeviceInfo) error {
	if self.count == 0 {
		handle, err := info.Open()
		if err != nil {
			return err
		}
		self.handle = handle
	}
	self.count += 1
	return nil
}

func (self HidHandle) close() error {
	if self.count == 1 {
		err := self.handle.Close()
		if err != nil {
			return err
		}
	}
	if self.count > 1 {
		self.count -= 1
	}
	return nil
}

type HidTransport struct {
	info       hid.DeviceInfo
	hid        HidHandle
	hidVersion int
	protocol   Protocol
}

func HidTransportNew(info hid.DeviceInfo) (HidTransport, error) {
	forceV1, found := os.LookupEnv("TREZOR_TRANSPORT_V1")
	var protocol Protocol
	if isTrezor2(info) || found && forceV1 != "1" {
		protocol, err := ProtocolV2New()
		if err != nil {
			return HidTransport{}, err
		}
	} else {
		protocol, err := ProtocolV1New()
		if err != nil {
			return HidTransport{}, err
		}
	}
	return HidTransport{
		hid: HidHandle{
			count:  0,
			handle: nil,
		},
		protocol: protocol,
	}, nil
}

func (self HidTransport) open() error {
	err := self.hid.open(self.info)
	if err != nil {
		return err
	}
	if isTrezor1(self.info) {
		if self.hidVersion, err = probeHidVersion(self); err != nil {
			return err
		}
	} else {
		self.hidVersion = 1
	}
	self.protocol.sessionBegin(self)
	return nil
}

func probeHidVersion(self HidTransport) (int, error) {
	data := [65]byte{}
	data[0] = 0
	data[1] = 63
	for i := 2; i < len(data); i++ {
		data[i] = 0xFF
	}
	{
		n, err := self.hid.handle.Write(data[:])
		if err != nil {
			return 0, err
		}
		if n == 65 {
			return 2, nil
		}
	}
	{
		n, err := self.hid.handle.Write(data[1:])
		if err != nil {
			return 0, err
		}
		if n == 64 {
			return 1, nil
		}
	}
	return 0, fmt.Errorf("Unknown HID version")
}

func (self HidTransport) close() error {
	self.protocol.sessionEnd(self)
	err := self.hid.close()
	if err != nil {
		return err
	}
	self.hidVersion = -1
	return nil
}

func (self HidTransport) read() (int32, []byte, error) {
	return self.protocol.read(self)
}

func (self HidTransport) readChunk() ([]byte, error) {
	chunk := [64]byte{}
	for {
		read, err := self.hid.handle.Read(chunk[:])
		if err != nil {
			return nil, err
		}
		if read != 64 {
			return nil, fmt.Errorf("Unexpected chunk size %d", read)
		}
		break
	}
	return chunk[:], nil
}

func (self HidTransport) write(message proto.Message) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	self.protocol.write(self, MessageType_value["MessageType_"+reflect.TypeOf(message).Name()], data)
	return nil
}

func (self HidTransport) writeChunk(chunk []byte) error {
	if len(chunk) != 64 {
		return fmt.Errorf("Unexpected chunk size: %d", len(chunk))
	}
	if self.hidVersion == 2 {
		if _, err := self.hid.handle.Write(append([]byte{0}, chunk...)); err != nil {
			return err
		}
	} else {
		if _, err := self.hid.handle.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}
