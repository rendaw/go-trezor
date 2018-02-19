package trezor

type Protocol interface {
	sessionBegin(transport Transport) error
	sessionEnd(transport Transport) error
	read(transport Transport) (int32, []byte, error)
	write(transport Transport, messageType int32, data []byte) error
}
