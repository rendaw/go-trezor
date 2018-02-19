package trezor

import (
	"fmt"
	"os"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/karalabe/hid"
)

func DevTrezor1() (uint16, uint16)   { return 0x534c, 0x0001 }
func DevTrezor2() (uint16, uint16)   { return 0x1209, 0x53c1 }
func DevTrezor2BL() (uint16, uint16) { return 0x1209, 0x53c0 }

func IsWirelink(dev hid.DeviceInfo) bool {
	return dev.UsagePage == 0xFF00 || dev.Interface == 0
}

func IsDebuglink(dev hid.DeviceInfo) bool {
	return dev.UsagePage == 0xFF01 || dev.Interface == 1
}

func IsTrezor1(dev hid.DeviceInfo) bool {
	gotVendor, gotProduct := DevTrezor1()
	return dev.VendorID == gotVendor && dev.ProductID == gotProduct
}
func IsTrezor2(dev hid.DeviceInfo) bool {
	gotVendor, gotProduct := DevTrezor2()
	return dev.VendorID == gotVendor && dev.ProductID == gotProduct
}
func IsTrezor2BL(dev hid.DeviceInfo) bool {
	gotVendor, gotProduct := DevTrezor2BL()
	return dev.VendorID == gotVendor && dev.ProductID == gotProduct
}

func Enumerate() ([]HidTransport, error) {
	out := []HidTransport{}
	for _, devList := range [][]hid.DeviceInfo{
		hid.Enumerate(DevTrezor1()),
		hid.Enumerate(DevTrezor2()),
		hid.Enumerate(DevTrezor2BL()),
	} {
		for _, dev := range devList {
			if IsWirelink(dev) {
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

func (self HidHandle) Open(info hid.DeviceInfo) error {
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

func (self HidHandle) Close() error {
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
	if IsTrezor2(info) || found && forceV1 != "1" {
		var err error
		protocol, err = ProtocolV2New()
		if err != nil {
			return HidTransport{}, err
		}
	} else {
		var err error
		protocol, err = ProtocolV1New()
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

func (self HidTransport) Open() error {
	err := self.hid.Open(self.info)
	if err != nil {
		return err
	}
	if IsTrezor1(self.info) {
		if self.hidVersion, err = ProbeHidVersion(self); err != nil {
			return err
		}
	} else {
		self.hidVersion = 1
	}
	self.protocol.SessionBegin(self)
	return nil
}

func ProbeHidVersion(self HidTransport) (int, error) {
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

func (self HidTransport) Close() error {
	self.protocol.SessionEnd(self)
	err := self.hid.Close()
	if err != nil {
		return err
	}
	self.hidVersion = -1
	return nil
}

func (self HidTransport) Read() (MessageType, []byte, error) {
	return self.protocol.Read(self)
}

func (self HidTransport) ReadChunk() ([]byte, error) {
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

func (self HidTransport) Write(message proto.Message) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	self.protocol.Write(self, MessageType(MessageType_value["MessageType_"+reflect.TypeOf(message).Name()]), data)
	return nil
}

func (self HidTransport) WriteChunk(chunk []byte) error {
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
