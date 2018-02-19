package trezor

type Transport interface {
	// Used by Protocol
	readChunk() ([]byte, error)
	writeChunk([]byte) error
}
