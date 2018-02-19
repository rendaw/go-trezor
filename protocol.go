package trezor

type Protocol interface {
	SessionBegin(transport Transport) error
	SessionEnd(transport Transport) error
	Read(transport Transport) (MessageType, []byte, error)
	Write(transport Transport, messageType MessageType, data []byte) error
}
