package utils

import (
	"encoding/binary"
	"github.com/gage-technologies/gigo-lib/utils"
)

func GenerateInternalServicePassword(id int64) (string, error) {
	// create slice to hold bytes from id
	buf := make([]byte, 8)

	// write the bytes of the int in big endian to the buffer
	binary.BigEndian.PutUint64(buf[:], uint64(id))

	// hash the buffer and return
	return utils.HashData(buf)
}
