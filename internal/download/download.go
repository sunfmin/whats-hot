// Package download is the Managed Download client: transfers the tool initiates itself
// and can therefore auto-resume on a Stall via HTTP Range requests. See docs/adr/
// 0001-observer-not-controller.md — this is the only kind of transfer the tool can
// genuinely resume; other apps' transfers are merely observed.
package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ProgressFunc reports cumulative bytes on disk and the total (or -1 if unknown).
type ProgressFunc func(downloaded, total int64)

// Options configures a Managed Download.
type Options struct {
	URL              string
	Dest             string        // output path; derived from the URL when empty
	StallBytesPerSec int64         // throughput below this counts toward a Stall (default 1024)
	StallWindow      time.Duration // time under threshold before resuming (default 10s)
	MaxRetries       int           // resume attempts after the first (default 5)
	Client           *http.Client
}

func (o *Options) applyDefaults() {
	if o.StallBytesPerSec <= 0 {
		o.StallBytesPerSec = 1024
	}
	if o.StallWindow <= 0 {
		o.StallWindow = 10 * time.Second
	}
	if o.MaxRetries <= 0 {
		o.MaxRetries = 5
	}
	if o.Client == nil {
		o.Client = http.DefaultClient
	}
	if o.Dest == "" {
		o.Dest = fileNameFromURL(o.URL)
	}
}

// Result summarises a completed (or failed) download.
type Result struct {
	Path     string
	Bytes    int64
	Resumed  bool
	Attempts int
}

// Get downloads o.URL to o.Dest, resuming from whatever is already on disk and
// auto-resuming across Stalls until the file is complete or retries are exhausted.
func Get(ctx context.Context, o Options, progress ProgressFunc) (*Result, error) {
	o.applyDefaults()

	f, err := os.OpenFile(o.Dest, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	res := &Result{Path: o.Dest, Bytes: offset}
	var lastErr error
	backoff := time.Second
	for attempt := 1; attempt <= o.MaxRetries+1; attempt++ {
		res.Attempts = attempt
		if offset > 0 {
			res.Resumed = true
		}
		n, total, ferr := o.fetch(ctx, f, offset, progress)
		offset += n
		res.Bytes = offset
		if ferr == nil && (total < 0 || offset >= total) {
			return res, nil
		}
		if ferr == nil {
			ferr = io.ErrUnexpectedEOF // short read: resume the remainder
		}
		lastErr = ferr
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if attempt <= o.MaxRetries {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return res, ctx.Err()
			}
			backoff *= 2
			offset, _ = f.Seek(0, io.SeekEnd)
		}
	}
	return res, fmt.Errorf("download failed after %d attempts: %w", res.Attempts, lastErr)
}

// fetch performs one HTTP attempt, appending to f from the given offset. It returns the
// bytes written this attempt and the known total size (-1 if unknown).
func (o Options) fetch(ctx context.Context, f *os.File, offset int64, progress ProgressFunc) (int64, int64, error) {
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, o.URL, nil)
	if err != nil {
		return 0, -1, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return 0, -1, err
	}
	defer resp.Body.Close()

	total := resp.ContentLength
	switch resp.StatusCode {
	case http.StatusOK:
		if offset > 0 { // server ignored Range — restart the file from scratch
			if err := f.Truncate(0); err != nil {
				return 0, -1, err
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return 0, -1, err
			}
			offset = 0
		}
	case http.StatusPartialContent:
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return 0, -1, err
		}
		if t := totalFromContentRange(resp.Header.Get("Content-Range")); t > 0 {
			total = t
		} else if total >= 0 {
			total += offset // 206 Content-Length is the remaining length
		}
	default:
		return 0, -1, fmt.Errorf("unexpected status %s", resp.Status)
	}

	written, err := copyWithStall(cancel, f, resp.Body, o.StallWindow,
		minBytesPerWindow(o.StallBytesPerSec, o.StallWindow), offset, total, progress)
	return written, total, err
}

// copyWithStall copies src->dst, invoking cancel() if fewer than minPerWindow bytes
// arrive within any StallWindow (which aborts the request so Get can resume).
func copyWithStall(cancel context.CancelFunc, dst io.Writer, src io.Reader, window time.Duration,
	minPerWindow int64, base, total int64, progress ProgressFunc) (int64, error) {

	var counter atomic.Int64
	done := make(chan struct{})
	defer close(done)

	go func() {
		t := time.NewTicker(window)
		defer t.Stop()
		var last int64
		for {
			select {
			case <-done:
				return
			case <-t.C:
				cur := counter.Load()
				if cur-last < minPerWindow {
					cancel() // Stall: too little progress this window
					return
				}
				last = cur
			}
		}
	}()

	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return written, werr
			}
			written += int64(n)
			counter.Add(int64(n))
			if progress != nil {
				progress(base+written, total)
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				return written, nil
			}
			return written, rerr
		}
	}
}

func minBytesPerWindow(perSec int64, window time.Duration) int64 {
	if v := int64(float64(perSec) * window.Seconds()); v > 0 {
		return v
	}
	return 1
}

func totalFromContentRange(cr string) int64 {
	// "bytes 500-999/1000"
	i := strings.LastIndexByte(cr, '/')
	if i < 0 {
		return -1
	}
	n, err := strconv.ParseInt(strings.TrimSpace(cr[i+1:]), 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func fileNameFromURL(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	base := path.Base(strings.TrimRight(u, "/"))
	if base == "" || base == "." || base == "/" {
		return "download"
	}
	return base
}
