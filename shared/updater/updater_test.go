package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatestTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","html_url":"https://github.com/will2469/argus/releases/tag/v1.0.0"}`))
	}))
	defer ts.Close()

	tag, err := FetchLatestTag(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v1.0.0" {
		t.Fatalf("expected v1.0.0, got %s", tag)
	}
}

func TestExtractFromTarGz(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("#!/bin/sh\necho argus\n")
	hdr := &tar.Header{
		Name:     "argus",
		Mode:     0755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
	tw.Close()
	gw.Close()

	extracted, err := extractFromTarGz(buf.Bytes(), "argus")
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if string(extracted) != string(content) {
		t.Fatalf("extracted content mismatch: got %s, want %s", string(extracted), string(content))
	}
}
