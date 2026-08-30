package common

import (
	"bytes"
	"encoding/gob"
)

// ToBytes encodes a value with encoding/gob.
//
// Note the gob rule that bites in practice: only EXPORTED fields are encoded. A
// struct with none at all fails with "type X has no exported fields" - which is why
// the record types in examples/db use exported field names.
func ToBytes(input any) ([]byte, error) {
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(input); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// FromBytes decodes gob-encoded bytes into output, which must be a pointer.
func FromBytes(data []byte, output any) error {
	return gob.NewDecoder(bytes.NewBuffer(data)).Decode(output)
}
