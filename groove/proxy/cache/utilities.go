package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
)

func objectToBytes(obj any) ([]byte, error) {
	var writeBuffer bytes.Buffer
	encoder := gob.NewEncoder(&writeBuffer)
	err := encoder.Encode(obj)
	if err != nil {
		log.Println(fmt.Errorf("Failed to encode cache entry %w", err))
		return nil, err
	}

	return writeBuffer.Bytes(), nil
}

func objectFromBytes(readBuffer []byte, obj any) error {
	decoder := gob.NewDecoder(bytes.NewReader(readBuffer))
	err := decoder.Decode(obj)
	if err != nil {
		log.Println(fmt.Errorf("Failed to decode cache entry %w", err))
		return err
	}
	return nil
}
