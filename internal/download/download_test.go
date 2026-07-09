package download

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func makeData(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('A' + i%26)
	}
	return b
}

func rangeStart(h string) int64 {
	h = strings.TrimPrefix(h, "bytes=")
	if i := strings.IndexByte(h, '-'); i >= 0 {
		h = h[:i]
	}
	n, _ := strconv.ParseInt(h, 10, 64)
	return n
}

func TestGetPlain(t *testing.T) {
	data := makeData(3000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.bin")
	res, err := Get(context.Background(), Options{URL: srv.URL, Dest: dest}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempts != 1 || res.Resumed {
		t.Errorf("attempts=%d resumed=%v, want 1/false", res.Attempts, res.Resumed)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Errorf("content mismatch: got %d bytes, want %d", len(got), len(data))
	}
}

func TestResumeFromExistingFile(t *testing.T) {
	data := makeData(3000)
	reader := bytes.NewReader(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "out.bin", time.Unix(0, 0), reader)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.bin")
	// pre-seed the first half on disk, as if a prior run got interrupted
	if err := os.WriteFile(dest, data[:1200], 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Get(context.Background(), Options{URL: srv.URL, Dest: dest}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed {
		t.Error("expected Resumed=true when a partial file exists")
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Errorf("resumed content mismatch: got %d bytes, want %d", len(got), len(data))
	}
}

func TestStallThenResume(t *testing.T) {
	data := makeData(4000)
	half := 2000
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		fl, _ := w.(http.Flusher)
		if n == 1 {
			// serve the first half, then hang until the client gives up (Stall)
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusOK)
			w.Write(data[:half])
			if fl != nil {
				fl.Flush()
			}
			<-r.Context().Done()
			return
		}
		// resume: honour the Range with a 206 for the remainder
		start := rangeStart(r.Header.Get("Range"))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data))-start, 10))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start:])
		if fl != nil {
			fl.Flush()
		}
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.bin")
	res, err := Get(context.Background(), Options{
		URL:              srv.URL,
		Dest:             dest,
		StallWindow:      100 * time.Millisecond,
		StallBytesPerSec: 1 << 20, // 1 MB/s — the 2 KB stall is far below this
		MaxRetries:       5,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempts < 2 || !res.Resumed {
		t.Errorf("attempts=%d resumed=%v, want >=2 and resumed", res.Attempts, res.Resumed)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Errorf("content after resume mismatch: got %d bytes, want %d", len(got), len(data))
	}
}
