package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
)

type plan struct {
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Validator string `json:"validator"`
	Parts     int    `json:"parts"`
}

type segment struct{ start, end int64 }

var errRangeIgnored = errors.New("server ignored the requested byte range")

func (p plan) segment(i int) segment {
	width := p.Size / int64(p.Parts)
	start := int64(i) * width
	if i == p.Parts-1 {
		return segment{start, p.Size - 1}
	}
	return segment{start, start + width - 1}
}

func newClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.DisableCompression = true
	return &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		if _, err := parseURL(req.URL.String()); err != nil {
			return errors.New("download was redirected to an unsupported URL")
		}
		return nil
	}}
}

func (q *Queue) request(ctx context.Context, rawURL, byteRange, validator string) (*http.Response, error) {
	ctx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return nil, errors.New("could not prepare download request")
	}
	req.Header.Set("Accept-Encoding", "identity")
	if byteRange != "" {
		req.Header.Set("Range", "bytes="+byteRange)
	}
	if validator != "" {
		req.Header.Set("If-Range", validator)
	}
	resp, err := q.client.Do(req)
	if err != nil {
		cancel()
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			// The URL in net/http errors may contain private query parameters.
			err = urlErr.Err
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	resp.Body = &bodyWithTimeout{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type bodyWithTimeout struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *bodyWithTimeout) Read(p []byte) (int, error) {
	timer := time.AfterFunc(30*time.Second, b.cancel)
	defer timer.Stop()
	return b.ReadCloser.Read(p)
}

func (b *bodyWithTimeout) Close() error {
	b.cancel()
	return b.ReadCloser.Close()
}

func (q *Queue) transfer(ctx context.Context, run *attempt, job *record) error {
	dir := partialDirectory(job)
	assembly := filepath.Join(dir, "assembled")
	if job.FinalPath != "" && sameFile(job.FinalPath, assembly) {
		return q.complete(ctx, job, job.FinalPath, assembly)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	resp, err := q.request(ctx, job.URL, "0-0", "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	name := responseName(resp)
	var p *plan
	switch resp.StatusCode {
	case http.StatusOK:
		return q.stream(ctx, run, job, dir, assembly, resp, false)
	case http.StatusPartialContent:
		first, last, size, ok := contentRange(resp.Header.Get("Content-Range"))
		if !ok || first != 0 || last != 0 || !identityResponse(resp) {
			return errors.New("server returned an invalid byte range")
		}
		validator := responseValidator(resp)
		_ = resp.Body.Close()
		if validator != "" {
			parts := 1
			if size >= 4<<20 {
				parts = 4
			}
			p = &plan{URL: resp.Request.URL.String(), Size: size, Validator: validator, Parts: parts}
		} else {
			return q.streamFull(ctx, run, job, dir, assembly)
		}
	case http.StatusRequestedRangeNotSatisfiable:
		if resp.Header.Get("Content-Range") != "bytes */0" {
			return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
		}
		return q.stream(ctx, run, job, dir, assembly, resp, true)
	default:
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	previousPlan := job.Plan
	if previousPlan == nil || *previousPlan != *p {
		if err := clearParts(dir); err != nil {
			return err
		}
	}
	sizes := make([]int64, p.Parts)
	var existing int64
	connections := 0
	for i := range sizes {
		span := p.segment(i)
		filename := partPath(dir, i)
		if info, err := os.Stat(filename); err == nil {
			if info.Size() > span.end-span.start+1 {
				if err := os.Remove(filename); err != nil {
					return err
				}
			} else {
				sizes[i] = info.Size()
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		existing += sizes[i]
		if sizes[i] < span.end-span.start+1 {
			connections++
		}
	}
	if err := q.prepare(ctx, run, job, p, name, new(p.Size), existing, connections); err != nil {
		return err
	}
	if err := q.downloadParts(ctx, job, dir, p, sizes); errors.Is(err, errRangeIgnored) {
		return q.streamFull(ctx, run, job, dir, assembly)
	} else if err != nil {
		return err
	}
	if err := joinParts(ctx, assembly, dir, p); err != nil {
		return err
	}
	return q.complete(ctx, job, "", assembly)
}

func (q *Queue) streamFull(ctx context.Context, run *attempt, job *record, dir, assembly string) error {
	resp, err := q.request(ctx, job.URL, "", "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d for the full download", resp.StatusCode)
	}
	return q.stream(ctx, run, job, dir, assembly, resp, false)
}

func (q *Queue) stream(ctx context.Context, run *attempt, job *record, dir, assembly string, resp *http.Response, empty bool) error {
	if !empty && !identityResponse(resp) {
		return errors.New("server sent an unexpected content encoding")
	}
	var body io.Reader = resp.Body
	var total *int64
	if empty {
		body, total = strings.NewReader(""), new(int64(0))
	} else if resp.ContentLength >= 0 {
		total = new(resp.ContentLength)
	}
	if err := clearParts(dir); err != nil {
		return err
	}
	if err := q.prepare(ctx, run, job, nil, responseName(resp), total, 0, 1); err != nil {
		return err
	}
	if err := q.withFile(assembly, os.O_TRUNC, func(f *os.File) error {
		n, err := io.Copy(progressWriter{f, q, job}, body)
		if err == nil && total != nil && n != *total {
			return io.ErrUnexpectedEOF
		}
		return err
	}); err != nil {
		return err
	}
	return q.complete(ctx, job, "", assembly)
}

func (q *Queue) prepare(ctx context.Context, run *attempt, job *record, p *plan, name string, total *int64, received int64, connections int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	job.Plan, job.Total, job.Name = p, total, name
	job.Received, job.Connections = received, connections
	job.FinalPath = ""
	run.base, run.started = received, time.Now()
	return q.saveLocked()
}

func (q *Queue) downloadParts(ctx context.Context, job *record, dir string, p *plan, sizes []int64) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup
	var firstErr error
	var fail sync.Once
	for i, size := range sizes {
		span := p.segment(i)
		start := span.start + size
		if start > span.end {
			continue
		}
		workers.Go(func() {
			if err := q.downloadPart(ctx, job, partPath(dir, i), p, start, span.end, size); err != nil {
				fail.Do(func() { firstErr = err; cancel() })
			}
			q.mu.Lock()
			if job.Connections > 0 {
				job.Connections--
			}
			q.mu.Unlock()
		})
	}
	workers.Wait()
	return firstErr
}

func (q *Queue) downloadPart(ctx context.Context, job *record, filename string, p *plan, start, end, originalSize int64) error {
	resp, err := q.request(ctx, p.URL, fmt.Sprintf("%d-%d", start, end), p.Validator)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return errRangeIgnored
	}
	first, last, size, ok := contentRange(resp.Header.Get("Content-Range"))
	if resp.StatusCode != http.StatusPartialContent || !ok || first != start || last != end || size != p.Size ||
		resp.Request.URL.String() != p.URL || !matchesValidator(resp, p.Validator) || !identityResponse(resp) {
		return errors.New("server changed the file or rejected a byte range; retry to check for a new version")
	}
	length := end - start + 1
	if resp.ContentLength >= 0 && resp.ContentLength != length {
		return errors.New("server sent the wrong length for a byte range")
	}
	return q.withFile(filename, os.O_APPEND, func(f *os.File) error {
		if _, err := io.CopyN(progressWriter{f, q, job}, resp.Body, length); err != nil {
			return err
		}
		// Read to EOF so erroneous extra bytes cannot masquerade as a completed part.
		var extra [1]byte
		if n, err := resp.Body.Read(extra[:]); n > 0 || err != io.EOF {
			if truncateErr := f.Truncate(originalSize); truncateErr != nil {
				return truncateErr
			}
			q.mu.Lock()
			job.Received -= length
			q.mu.Unlock()
			return errors.New("server failed to end the byte range correctly")
		}
		return nil
	})
}

func (q *Queue) withFile(filename string, flag int, write func(*os.File) error) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|flag, 0600)
	if err != nil {
		return err
	}
	err = write(f)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

func joinParts(ctx context.Context, output, dir string, p *plan) (err error) {
	dst, err := os.Create(output)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dst.Close()) }()
	for i := 0; i < p.Parts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		src, err := os.Open(partPath(dir, i))
		if err != nil {
			return err
		}
		n, err := io.Copy(dst, checkedReader{ctx, src})
		if err := errors.Join(err, src.Close()); err != nil {
			return err
		}
		span := p.segment(i)
		if n != span.end-span.start+1 {
			return errors.New("a download part is incomplete")
		}
	}
	return dst.Sync()
}

type checkedReader struct {
	ctx context.Context
	r   io.Reader
}

func (r checkedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (q *Queue) complete(ctx context.Context, job *record, existing, assembly string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if existing == "" {
		ext := filepath.Ext(job.Name)
		base := strings.TrimSuffix(job.Name, ext)
		for i := 0; ; i++ {
			name := job.Name
			if i > 0 {
				name = fmt.Sprintf("%s (%d)%s", base, i, ext)
			}
			candidate := filepath.Join(job.Directory, name)
			if _, err := os.Lstat(candidate); err == nil {
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			job.FinalPath = candidate
			if err := q.saveLocked(); err != nil {
				return err
			}
			if err := os.Link(assembly, candidate); errors.Is(err, os.ErrExist) {
				continue
			} else if err != nil {
				return fmt.Errorf("save completed file: %w", err)
			}
			break
		}
	}
	job.State, job.Connections, job.Name = Completed, 0, filepath.Base(job.FinalPath)
	job.Total = new(job.Received)
	if err := q.saveLocked(); err != nil {
		job.State = Downloading
		return err
	}
	_ = os.RemoveAll(partialDirectory(job))
	return nil
}

func sameFile(a, b string) bool {
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	return err == nil && os.SameFile(left, right)
}

func responseValidator(resp *http.Response) string {
	if tag := resp.Header.Get("ETag"); tag != "" {
		if len(tag) >= 2 && tag[0] == '"' && tag[len(tag)-1] == '"' && !strings.ContainsRune(tag[1:len(tag)-1], '"') {
			return tag
		}
		return ""
	}
	modified, err := http.ParseTime(resp.Header.Get("Last-Modified"))
	if err != nil {
		return ""
	}
	date, err := http.ParseTime(resp.Header.Get("Date"))
	if err == nil && date.Sub(modified) >= time.Minute {
		return modified.UTC().Format(http.TimeFormat)
	}
	return ""
}

func matchesValidator(resp *http.Response, expected string) bool {
	// A successful If-Range response may omit representation metadata.
	if strings.HasPrefix(expected, `"`) {
		return resp.Header.Get("ETag") == "" || resp.Header.Get("ETag") == expected
	}
	if value := resp.Header.Get("Last-Modified"); value != "" {
		modified, err := http.ParseTime(value)
		return err == nil && modified.UTC().Format(http.TimeFormat) == expected
	}
	return true
}

func identityResponse(resp *http.Response) bool {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	return encoding == "" || encoding == "identity"
}

func contentRange(header string) (first, last, size int64, valid bool) {
	digits, ok := strings.CutPrefix(header, "bytes ")
	if !ok {
		return
	}
	span, length, ok := strings.Cut(digits, "/")
	if !ok {
		return
	}
	start, end, ok := strings.Cut(span, "-")
	if !ok {
		return
	}
	first, e1 := strconv.ParseInt(start, 10, 64)
	last, e2 := strconv.ParseInt(end, 10, 64)
	size, e3 := strconv.ParseInt(length, 10, 64)
	return first, last, size, e1 == nil && e2 == nil && e3 == nil && first >= 0 && last >= first && size > last
}

func responseName(resp *http.Response) string {
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && params["filename"] != "" {
		return safeFilename(params["filename"])
	}
	return safeFilename(path.Base(resp.Request.URL.Path))
}

func safeFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || strings.ContainsRune(`<>:"|?*/\`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" || name == "." || name == ".." {
		return "download"
	}
	stem, _, _ := strings.Cut(name, ".")
	winName := strings.ToUpper(strings.TrimSpace(stem))
	reserved := " CON PRN AUX NUL CONIN$ CONOUT$ COM0 COM1 COM2 COM3 COM4 COM5 COM6 COM7 COM8 COM9 COM¹ COM² COM³ LPT0 LPT1 LPT2 LPT3 LPT4 LPT5 LPT6 LPT7 LPT8 LPT9 LPT¹ LPT² LPT³ "
	if strings.Contains(reserved, " "+winName+" ") {
		name = "_" + name
	}
	if len(name) > 200 || len(utf16.Encode([]rune(name))) > 200 {
		ext := filepath.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		for i := range name {
			if i >= 175 {
				return strings.TrimRight(name[:i], " .") + ext
			}
		}
	}
	return name
}

func partialDirectory(job *record) string { return filepath.Join(job.Directory, ".jisu-"+job.ID) }
func partPath(dir string, i int) string   { return filepath.Join(dir, fmt.Sprintf("part-%d", i)) }

func clearParts(dir string) error {
	for i := 0; i < 4; i++ {
		if err := os.Remove(partPath(dir, i)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

type progressWriter struct {
	io.Writer
	q   *Queue
	job *record
}

func (w progressWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.q.mu.Lock()
	w.job.Received += int64(n)
	w.q.mu.Unlock()
	return n, err
}
