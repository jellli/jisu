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
	"time"
	"unicode"
	"unicode/utf16"
)

type plan struct {
	URL       string      `json:"url"`
	Size      int64       `json:"size"`
	Validator string      `json:"validator"`
	Parts     int         `json:"parts,omitempty"`
	Ranges    []byteRange `json:"ranges,omitempty"`
}

type byteRange struct {
	Start int64 `json:"start"`
	Next  int64 `json:"next"`
	End   int64 `json:"end"`
}

var minimumRangeSize int64 = 256 << 10

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
		for _, previous := range via {
			if previous.URL.Hostname() != req.URL.Hostname() || (previous.URL.Scheme == "https" && req.URL.Scheme == "http") {
				req.Header.Del("Cookie")
			}
		}
		if ref, _ := url.Parse(req.Referer()); ref != nil && ref.Scheme == "https" && req.URL.Scheme == "http" {
			req.Header.Del("Referer")
		}
		if _, err := parseURL(req.URL.String()); err != nil {
			return errors.New("download was redirected to an unsupported URL")
		}
		return nil
	}}
}

func (q *Queue) request(ctx context.Context, rawURL, byteRange, validator string, options ...RequestOptions) (*http.Response, error) {
	if throttle, ok := ctx.Value(throttleKey{}).(*requestThrottle); ok {
		return throttle.request(ctx, q, rawURL, byteRange, validator, options...)
	}
	return q.requestOnce(ctx, rawURL, byteRange, validator, options...)
}

func (q *Queue) requestOnce(ctx context.Context, rawURL, byteRange, validator string, options ...RequestOptions) (*http.Response, error) {
	ctx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return nil, errors.New("could not prepare download request")
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "jisu")
	if len(options) > 0 {
		options[0].apply(req)
	}
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
	ctx = context.WithValue(ctx, throttleKey{}, newRequestThrottle())
	dir := partialDirectory(job)
	assembly := filepath.Join(dir, "assembled")
	if job.FinalPath != "" && sameFile(job.FinalPath, assembly) {
		return q.complete(ctx, job, job.FinalPath, assembly)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	resp, err := q.request(ctx, job.URL, "0-0", "", job.Options)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	name := responseName(resp, job.Options.Filename)
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
			p = &plan{URL: resp.Request.URL.String(), Size: size, Validator: validator}
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
	previous := job.Plan
	restart := previous == nil || previous.URL != p.URL || previous.Size != p.Size || previous.Validator != p.Validator
	if !restart && len(previous.Ranges) > 0 {
		info, err := os.Stat(assembly)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		restart = errors.Is(err, os.ErrNotExist)
		if !restart {
			for _, r := range previous.Ranges {
				if info.Size() < r.Next {
					restart = true
					break
				}
			}
		}
	}
	if restart {
		if err := clearParts(dir); err != nil {
			return err
		}
		if err := os.Remove(assembly); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if len(previous.Ranges) > 0 {
		p.Ranges = append([]byteRange(nil), previous.Ranges...)
	} else if previous.Parts > 0 {
		if err := migrateParts(ctx, dir, assembly, previous, p); err != nil {
			return err
		}
	}
	if len(p.Ranges) == 0 {
		q.mu.Lock()
		count := q.connectionTargetLocked(job)
		q.mu.Unlock()
		if p.Size < 4<<20 {
			count = 1
		}
		for i := 0; i < count; i++ {
			start, end := int64(i)*p.Size/int64(count), int64(i+1)*p.Size/int64(count)-1
			p.Ranges = append(p.Ranges, byteRange{start, start, end})
		}
	}
	var existing int64
	for _, r := range p.Ranges {
		existing += r.Next - r.Start
	}
	if err := q.prepare(ctx, run, job, p, name, new(p.Size), existing, 0); err != nil {
		return err
	}
	if err := clearParts(dir); err != nil {
		return err
	}
	if err := q.downloadRanges(ctx, run, job, assembly, p); errors.Is(err, errRangeIgnored) {
		return q.streamFull(ctx, run, job, dir, assembly)
	} else if err != nil {
		return err
	}
	return q.complete(ctx, job, "", assembly)
}

func migrateParts(ctx context.Context, dir, assembly string, previous, p *plan) (err error) {
	f, err := os.OpenFile(assembly, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	for i := 0; i < previous.Parts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		span := previous.segment(i)
		r := byteRange{span.start, span.start, span.end}
		src, e := os.Open(partPath(dir, i))
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if e == nil {
			info, statErr := src.Stat()
			var copyErr error
			if statErr == nil && info.Size() <= span.end-span.start+1 {
				n, err := io.Copy(io.NewOffsetWriter(f, span.start), checkedReader{ctx, src})
				r.Next += n
				copyErr = err
			}
			if err := errors.Join(statErr, copyErr, src.Close()); err != nil {
				return err
			}
		}
		p.Ranges = append(p.Ranges, r)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.Sync()
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

func (q *Queue) streamFull(ctx context.Context, run *attempt, job *record, dir, assembly string) error {
	resp, err := q.request(ctx, job.URL, "", "", job.Options)
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
	if err := q.prepare(ctx, run, job, nil, responseName(resp, job.Options.Filename), total, 0, 1); err != nil {
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
	run.ranges = nil
	return q.saveLocked()
}

type liveRange struct {
	byteRange
	busy bool
}

func (q *Queue) downloadRanges(ctx context.Context, run *attempt, job *record, filename string, p *plan) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	ranges := make([]*liveRange, 0, len(p.Ranges))
	for _, r := range p.Ranges {
		ranges = append(ranges, &liveRange{byteRange: r})
	}
	q.mu.Lock()
	run.ranges = ranges
	q.mu.Unlock()
	checkpoint := func() error {
		q.mu.Lock()
		defer q.mu.Unlock()
		if err := f.Sync(); err != nil {
			return err
		}
		saved := *p
		saved.Ranges = make([]byteRange, len(ranges))
		for i, r := range ranges {
			saved.Ranges[i] = r.byteRange
			if r.busy && r.Next > r.End {
				saved.Ranges[i].Next = r.End
			}
		}
		job.Plan = &saved
		return q.saveLocked()
	}
	type result struct {
		id  int
		err error
	}
	done := make(chan result, 32)
	type worker struct {
		cancel  context.CancelFunc
		stopped bool
	}
	active := make(map[int]*worker)
	nextID := 0
	var firstErr error
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	stopping := false
	for {
		q.mu.Lock()
		target := q.connectionTargetLocked(job)
		if p.Size < 4<<20 {
			target = 1
		}
		if stopping {
			target = 0
		}
		excess := len(active) - target
		for _, w := range active {
			if w.stopped {
				excess--
			}
		}
		for _, w := range active {
			if excess <= 0 {
				break
			}
			if w.stopped {
				continue
			}
			w.cancel()
			w.stopped = true
			excess--
		}
		for len(active) < target {
			var assigned *liveRange
			for _, r := range ranges {
				if !r.busy && r.Next <= r.End {
					assigned = r
					break
				}
			}
			if assigned == nil && p.Size >= 4<<20 {
				var largest *liveRange
				for _, r := range ranges {
					if r.End-r.Next+1 >= 2*minimumRangeSize && (largest == nil || r.End-r.Next > largest.End-largest.Next) {
						largest = r
					}
				}
				if largest != nil {
					mid := largest.Next + (largest.End-largest.Next+1)/2
					assigned = &liveRange{byteRange: byteRange{mid, mid, largest.End}}
					largest.End = mid - 1
					ranges = append(ranges, assigned)
					run.ranges = ranges
				}
			}
			if assigned == nil {
				break
			}
			assigned.busy = true
			workerCtx, stop := context.WithCancel(ctx)
			id := nextID
			nextID++
			active[id] = &worker{cancel: stop}
			job.Connections++
			go func(r *liveRange) {
				err := q.downloadRange(workerCtx, job, f, p, r)
				q.mu.Lock()
				r.busy = false
				if job.Connections > 0 {
					job.Connections--
				}
				q.mu.Unlock()
				done <- result{id, err}
			}(assigned)
		}
		q.mu.Unlock()
		if len(active) == 0 {
			break
		}
		select {
		case result := <-done:
			active[result.id].cancel()
			delete(active, result.id)
			if result.err != nil && !errors.Is(result.err, context.Canceled) && ctx.Err() == nil {
				if firstErr == nil {
					firstErr = result.err
				}
				stopping = true
				cancel()
			}
			if err := checkpoint(); err != nil && firstErr == nil {
				firstErr = err
				stopping = true
				cancel()
			}
		case <-run.changed:
		case <-ticker.C:
			if err := checkpoint(); err != nil && firstErr == nil {
				firstErr = err
				stopping = true
				cancel()
			}
		case <-ctx.Done():
			stopping = true
		}
	}
	if err := checkpoint(); firstErr == nil {
		firstErr = err
	}
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

func (q *Queue) downloadRange(ctx context.Context, job *record, f *os.File, p *plan, r *liveRange) error {
	q.mu.Lock()
	start, end := r.Next, r.End
	q.mu.Unlock()
	opts := job.Options
	original, _ := url.Parse(job.URL)
	target, _ := url.Parse(p.URL)
	if original.Hostname() != target.Hostname() || (original.Scheme == "https" && target.Scheme == "http") {
		opts.Cookie = ""
	}
	resp, err := q.request(ctx, p.URL, fmt.Sprintf("%d-%d", start, end), p.Validator, opts)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return errRangeIgnored
	}
	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	first, last, size, ok := contentRange(resp.Header.Get("Content-Range"))
	if resp.StatusCode != http.StatusPartialContent || !ok || first != start || last != end || size != p.Size ||
		resp.Request.URL.String() != p.URL || !matchesValidator(resp, p.Validator) || !identityResponse(resp) {
		return errors.New("server changed the file or rejected a byte range; retry to check for a new version")
	}
	if resp.ContentLength >= 0 && resp.ContentLength != end-start+1 {
		return errors.New("server sent the wrong length for a byte range")
	}
	buf := make([]byte, 32<<10)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		q.mu.Lock()
		remaining := r.End - r.Next + 1
		q.mu.Unlock()
		if remaining <= 0 {
			break
		}
		n, readErr := resp.Body.Read(buf[:min(int64(len(buf)), remaining)])
		q.mu.Lock()
		n = int(min(int64(n), r.End-r.Next+1))
		written, writeErr := f.WriteAt(buf[:n], r.Next)
		r.Next += int64(written)
		job.Received += int64(written)
		left := r.End - r.Next + 1
		q.mu.Unlock()
		if writeErr != nil {
			return writeErr
		}
		if readErr != nil && left > 0 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return io.ErrUnexpectedEOF
		}
	}
	q.mu.Lock()
	full := r.End == end
	q.mu.Unlock()
	if full {
		var extra [1]byte
		if n, err := resp.Body.Read(extra[:]); n > 0 || err != io.EOF {
			q.mu.Lock()
			job.Received -= r.Next - start
			r.Next = start
			q.mu.Unlock()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.New("server failed to end the byte range correctly")
		}
	}
	return nil
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

func responseName(resp *http.Response, filenameHint string) string {
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && params["filename"] != "" {
		return safeFilename(params["filename"])
	}
	if filenameHint != "" {
		return safeFilename(filenameHint)
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "part-") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
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
