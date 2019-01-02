package helpers

import (
	"bytes"
	"encoding/pem"
)

// EncodePem using byte array
func EncodePem(headerType string, content []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := pem.Encode(buf, &pem.Block{Type: headerType, Bytes: content})
	return buf.Bytes(), err
}
