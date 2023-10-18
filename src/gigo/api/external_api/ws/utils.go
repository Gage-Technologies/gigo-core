package ws

import (
	"encoding/hex"
	"math/rand"
	"time"
)

// NewSequenceID
//
//	Generates a new sequence id.
func NewSequenceID() string {
	// generate a random hex string of 8 characters
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	var buf [8]byte
	for i := range buf {
		buf[i] = byte(r.Intn(16))
	}
	return hex.EncodeToString(buf[:])
}

// PrepMessage
//
//	Prepares a message for sending over the websocket.
func PrepMessage[T any](seqId string, msgType MessageType, payload T) Message[T] {
	// if sequence id is empty, generate one
	if seqId == "" {
		seqId = NewSequenceID()
	}

	return Message[T]{
		SequenceID: seqId,
		Type:       msgType,
		Payload:    payload,
	}
}
