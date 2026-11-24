package store

import "testing"

func TestNonEmptyLineCount(t *testing.T) {
	if got := nonEmptyLineCount("alpha\n\n beta \r\ngamma\n"); got != 3 {
		t.Fatalf("expected 3 entries, got %d", got)
	}
}

func TestListenerValidation(t *testing.T) {
	valid := []struct{ prefix, certificate, webdav string }{
		{"http://127.0.0.1:8080/", "", ""},
		{"https://localhost:8443/api/", "fixture-cert", "/dav"},
	}
	for _, value := range valid {
		if err := validateListener(value.prefix, value.certificate, value.webdav); err != nil {
			t.Fatalf("expected valid listener: %v", err)
		}
	}
	invalid := []struct{ prefix, certificate, webdav string }{
		{"http://localhost:8080/../admin/", "", ""},
		{"https://localhost:8443/", "", ""},
		{"file:///tmp/socket", "", ""},
		{"http://user:pass@localhost:8080/", "", ""},
		{"http://localhost:8080/", "", "../dav"},
	}
	for _, value := range invalid {
		if err := validateListener(value.prefix, value.certificate, value.webdav); err == nil {
			t.Fatalf("expected invalid listener: %#v", value)
		}
	}
}
