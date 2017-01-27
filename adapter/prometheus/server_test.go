package prometheus

import (
	"fmt"
	"net/http"
	"testing"
)

func TestServer(t *testing.T) {
	testAddr := "127.0.0.1:9992"
	s := newServer(testAddr)
	if err := s.Start(); err != nil {
		t.Fatalf("Start() failed unexpectedly: %v", err)
	}

	testURL := fmt.Sprintf("http://%s%s", testAddr, metricsPath)
	// verify a response is returned from "/metrics"
	resp, err := http.Get(testURL)
	if err != nil {
		t.Fatalf("Failed to retrieve '%s' path: %v", metricsPath, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("http.GET => %v, wanted '%v'", resp.StatusCode, http.StatusOK)
	}

	if err := s.Close(); err != nil {
		t.Errorf("Failed to close server properly: %v", err)
	}

	if resp, err := http.Get(testURL); err == nil {
		t.Errorf("http.GET should have failed for '%s'; got %v", metricsPath, resp)
	}
}
