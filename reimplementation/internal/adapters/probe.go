package adapters

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/research/specter-reimplementation/internal/domain"
)

type Result struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func Probe(ctx context.Context, adapter domain.Adapter, realEnabled bool) (Result, error) {
	if !adapter.Enabled {
		return Result{Status: "disabled", Detail: "adapter is disabled"}, nil
	}
	if adapter.Mode == "fixture" {
		return Result{Status: "healthy", Detail: "deterministic fixture responded"}, nil
	}
	if !realEnabled {
		return Result{}, errors.New("real adapters are disabled by configuration")
	}
	if adapter.Name == "postgresql" && adapter.Endpoint == "internal" {
		return Result{Status: "healthy", Detail: "database connection is managed by the server"}, nil
	}
	parsed, err := url.Parse(adapter.Endpoint)
	if err != nil || parsed.Host == "" {
		return Result{}, errors.New("adapter endpoint must be an absolute URL")
	}
	switch parsed.Scheme {
	case "http", "https":
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return Result{}, err
		}
		response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
		if err != nil {
			return Result{}, err
		}
		defer response.Body.Close()
		if response.StatusCode >= 500 {
			return Result{}, fmt.Errorf("endpoint returned %s", response.Status)
		}
		return Result{Status: "healthy", Detail: response.Status}, nil
	case "smtp", "ssh", "tor", "winrm", "tcp":
		host := parsed.Host
		if !strings.Contains(host, ":") {
			return Result{}, errors.New("TCP adapter endpoint must include a port")
		}
		connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", host)
		if err != nil {
			return Result{}, err
		}
		_ = connection.Close()
		return Result{Status: "healthy", Detail: "TCP connection accepted"}, nil
	default:
		return Result{}, fmt.Errorf("unsupported adapter scheme %q", parsed.Scheme)
	}
}
