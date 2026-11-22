package leetcode

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type RateLimit struct {
	Site    Site
	RetryAt time.Time
}

func (e *RateLimit) Error() string {
	return fmt.Sprintf("%s is rate limiting requests. Retry after %s.", e.Site, e.RetryAt.Local().Format("15:04:05"))
}

type StatusError struct {
	Site       Site
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d", e.Site, e.StatusCode)
}

func (c *Client) responseError(resp *http.Response) error {
	if resp.StatusCode != http.StatusTooManyRequests {
		return &StatusError{Site: c.Site, StatusCode: resp.StatusCode}
	}
	now := time.Now()
	at := now.Add(time.Minute)
	maxAt := now.Add(24 * time.Hour)
	header := resp.Header.Get("Retry-After")
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 && seconds <= 86400 {
		at = now.Add(time.Duration(seconds) * time.Second)
	} else if date, err := http.ParseTime(header); err == nil && date.After(now) {
		if date.After(maxAt) {
			at = maxAt
		} else {
			at = date
		}
	}
	return &RateLimit{Site: c.Site, RetryAt: at}
}
