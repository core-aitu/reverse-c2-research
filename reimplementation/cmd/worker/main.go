package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/research/specter-reimplementation/internal/worker"
)

func main() {
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 4<<20))
	decoder.DisallowUnknownFields()
	var request worker.Request
	if err := decoder.Decode(&request); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(worker.Response{Version: 1, Error: "invalid worker request"})
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(worker.Run(request))
}
