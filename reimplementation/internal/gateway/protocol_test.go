package gateway

import (
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestGoldenCheckinFrame(t *testing.T) {
	value := Message{Version: 1, ID: "sim-0001", Type: "simulator.checkin", Payload: json.RawMessage(`{"cursor":7}`)}
	frame, err := Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	const goldenHex = "534952310000004f7b2276657273696f6e223a312c226964223a2273696d2d30303031222c2274797065223a2273696d756c61746f722e636865636b696e222c227061796c6f6164223a7b22637572736f72223a377d7d"
	if got := hex.EncodeToString(frame); got != goldenHex {
		t.Fatalf("golden frame mismatch\n got: %s\nwant: %s", got, goldenHex)
	}
	decoded, err := Decode(frame)
	if err != nil || decoded.ID != value.ID || decoded.Type != value.Type {
		t.Fatalf("round trip failed: value=%#v err=%v", decoded, err)
	}
}

func TestRejectsUnknownMessageAndTrailingBytes(t *testing.T) {
	if _, err := Encode(Message{Version: 1, ID: "x", Type: "shell", Payload: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("unknown message type was accepted")
	}
	frame, err := Encode(Message{Version: 1, ID: "x", Type: "simulator.checkin", Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(append(frame, 0)); err == nil {
		t.Fatal("trailing frame bytes were accepted")
	}
}
