package httpapi

import "testing"

func TestParseObjectRange(t *testing.T) {
	tests := []struct {
		header             string
		start, end         int64
		partial, shouldErr bool
	}{
		{"", 0, 99, false, false},
		{"bytes=10-19", 10, 19, true, false},
		{"bytes=90-", 90, 99, true, false},
		{"bytes=-10", 90, 99, true, false},
		{"bytes=100-", 0, 0, false, true},
		{"bytes=1-2,4-5", 0, 0, false, true},
	}
	for _, test := range tests {
		start, end, partial, err := parseObjectRange(test.header, 100)
		if (err != nil) != test.shouldErr {
			t.Errorf("%q error=%v", test.header, err)
			continue
		}
		if err == nil && (start != test.start || end != test.end || partial != test.partial) {
			t.Errorf("%q got %d-%d partial=%v", test.header, start, end, partial)
		}
	}
}
