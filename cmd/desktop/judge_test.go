package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

func TestJudgeErrorNoticeAndSessionCacheInvalidation(t *testing.T) {
	apiErr := &leetcode.StatusError{Site: leetcode.CN, StatusCode: http.StatusForbidden}

	// Run errors should surface concrete causes without pointing to Submissions.
	if got := judgeErrorNotice(false, false, apiErr, ""); got != "leetcode.cn returned HTTP 403" {
		t.Fatalf("unexpected Run start notice: %q", got)
	}
	if got := judgeErrorNotice(false, true, context.Canceled, ""); got != "Canceled" {
		t.Fatalf("unexpected Run cancel notice: %q", got)
	}
	if got := judgeErrorNotice(false, true, context.DeadlineExceeded, ""); got != "Timed out waiting for test result" {
		t.Fatalf("unexpected Run timeout notice: %q", got)
	}
	if got := judgeErrorNotice(false, true, nil, "FAILURE"); got != "Judge returned FAILURE" {
		t.Fatalf("unexpected Run state notice: %q", got)
	}

	// Submit polling interruptions still remind the user to check Submissions.
	if got := judgeErrorNotice(true, true, context.Canceled, ""); got != "Stopped waiting — check Submissions" {
		t.Fatalf("unexpected Submit cancel notice: %q", got)
	}
	if got := judgeErrorNotice(true, true, context.DeadlineExceeded, ""); got != "Result unknown — check Submissions" {
		t.Fatalf("unexpected Submit timeout notice: %q", got)
	}
	if got := judgeErrorNotice(true, true, errors.New("network lost"), ""); got != "network lost — check Submissions" {
		t.Fatalf("unexpected Submit polling error notice: %q", got)
	}

	// 401/403 invalidates the site's cached session cookies; 500 does not.
	key := sessionCacheKey(leetcode.CN, "alice")
	sessionCacheMu.Lock()
	sessionCache[key] = verifiedSessionEntry{
		cookies:    []*http.Cookie{{Name: "LEETCODE_SESSION", Value: "tok"}},
		verifiedAt: time.Now(),
	}
	sessionCacheMu.Unlock()
	t.Cleanup(func() { invalidateSessionCache(leetcode.CN) })

	maybeInvalidateJudgeSession(leetcode.CN, &leetcode.StatusError{Site: leetcode.CN, StatusCode: http.StatusInternalServerError})
	sessionCacheMu.Lock()
	_, stillCached := sessionCache[key]
	sessionCacheMu.Unlock()
	if !stillCached {
		t.Fatal("500 error should not invalidate session cache")
	}

	maybeInvalidateJudgeSession(leetcode.CN, apiErr)
	sessionCacheMu.Lock()
	_, after403 := sessionCache[key]
	sessionCacheMu.Unlock()
	if after403 {
		t.Fatal("403 error should invalidate session cache")
	}
}

func TestJudgeNoticeViewSubmissionsLinkOnlyForSubmit(t *testing.T) {
	a, _ := testWorkspace(t)
	a.testPanelCollapsed = false
	a.resultTab = "result"
	v := ui.NewTester(a.view, 1360, 860)

	// Run failure shows exact notice without a View submissions link.
	a.judge = &judgeState{submit: false, notice: "leetcode.cn returned HTTP 403"}
	v.Frame()
	if !v.HasText("leetcode.cn returned HTTP 403") {
		t.Fatal("expected Run error notice text")
	}
	if v.HasText("View submissions ↗") {
		t.Fatal("Run failure should not render View submissions link")
	}

	// Submit failure shows notice with View submissions link.
	a.judge = &judgeState{submit: true, notice: "Result unknown — check Submissions"}
	v.Frame()
	if !v.HasText("Result unknown — check Submissions") || !v.HasText("View submissions ↗") {
		t.Fatal("Submit failure should render notice and View submissions link")
	}
}
