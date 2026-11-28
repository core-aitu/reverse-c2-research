package gateway

import (
	"testing"
	"time"
)

func FuzzDecodeGatewayFrames(f *testing.F) {
	validCheckin, _ := EncodeCheckin(Checkin{AgentID: "agent", Sequence: 1, Limit: 1, Timestamp: time.Now().UTC()})
	validBatch, _ := EncodeTaskBatch(TaskBatch{Sequence: 1})
	validResult, _ := EncodeResult(Result{TaskID: "task", SubmissionID: "submission", Sequence: 2, Success: true, Payload: []byte(`{}`)})
	validRegistration, _ := EncodeRegistration(Registration{Build: "fuzz", AgentID: "agent", Expiration: time.Now().UTC().Add(time.Hour)})
	validOutput, _ := EncodeTaskOutputChunk(TaskOutputChunk{TaskID: "task", SubmissionID: "submission", Sequence: 3, Stream: "stdout", Data: []byte("x")})
	for _, seed := range [][]byte{nil, {}, []byte("not-gzip"), validCheckin, validBatch, validResult, validRegistration, validOutput} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		if len(frame) > MaxFrameSize+1 {
			t.Skip()
		}
		_, _ = DecodeRegistration(frame)
		_, _ = DecodeCheckin(frame)
		_, _ = DecodeTaskBatch(frame)
		_, _ = DecodeResult(frame)
		_, _ = DecodeTaskOutputChunk(frame)
		_, _ = DecodeFileTransferManifest(frame)
		_, _ = DecodeFileTransferComplete(frame)
		_, _ = DecodeAck(frame)
		_, _ = DecodeReceipt(frame)
	})
}
