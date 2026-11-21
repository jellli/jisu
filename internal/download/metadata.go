package download

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Metadata struct {
	Name string
	Size *int64
}

func (q *Queue) Probe(ctx context.Context, rawURL string) (Metadata, error) {
	u, err := parseURL(strings.TrimSpace(rawURL))
	if err != nil {
		return Metadata{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := q.requestOnce(ctx, u.String(), "0-0", "")
	if err != nil {
		return Metadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return Metadata{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	result := Metadata{Name: responseName(resp, "")}
	if resp.StatusCode == http.StatusPartialContent {
		if start, end, total, ok := contentRange(resp.Header.Get("Content-Range")); ok && start == 0 && end == 0 {
			result.Size = &total
		}
	} else if resp.ContentLength >= 0 {
		result.Size = new(resp.ContentLength)
	}
	return result, nil
}
