package gateway

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

const (
	Version      = 1
	maxFrameSize = 1 << 20
)

var magic = [4]byte{'S', 'I', 'R', '1'}

type Message struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func Encode(value Message) ([]byte, error) {
	if err := validate(value); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(payload) > maxFrameSize {
		return nil, errors.New("gateway frame exceeds 1 MiB")
	}
	frame := make([]byte, 8+len(payload))
	copy(frame[:4], magic[:])
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame, nil
}

func Decode(frame []byte) (Message, error) {
	if len(frame) < 8 || !bytes.Equal(frame[:4], magic[:]) {
		return Message{}, errors.New("invalid gateway frame magic")
	}
	length := int(binary.BigEndian.Uint32(frame[4:8]))
	if length < 2 || length > maxFrameSize || len(frame) != 8+length {
		return Message{}, errors.New("invalid gateway frame length")
	}
	decoder := json.NewDecoder(bytes.NewReader(frame[8:]))
	decoder.DisallowUnknownFields()
	var value Message
	if err := decoder.Decode(&value); err != nil {
		return Message{}, errors.New("invalid gateway frame JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Message{}, errors.New("gateway frame contains trailing JSON")
	}
	if err := validate(value); err != nil {
		return Message{}, err
	}
	return value, nil
}

func validate(value Message) error {
	if value.Version != Version || !validID(value.ID) || !allowedType(value.Type) ||
		len(value.Payload) == 0 || !json.Valid(value.Payload) {
		return errors.New("invalid gateway message")
	}
	return nil
}

func allowedType(value string) bool {
	switch value {
	case "simulator.register", "simulator.checkin", "simulator.tasks", "simulator.result":
		return true
	default:
		return false
	}
}

func validID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character == '-' || character == '_' || character == '.' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}
