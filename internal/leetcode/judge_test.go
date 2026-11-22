package leetcode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestJudgeRequestsStayOnSelectedSite(t *testing.T) {
	const runID = "runcode_1732189234.0935075_7U4XoQAQyo"
	for _, site := range []Site{COM, CN} {
		client := New(site)
		p := Problem{ID: "1", Slug: "sample"}
		ctx := context.Background()
		for _, submit := range []bool{false, true} {
			client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != string(site) || r.Header.Get("Referer") != site.URL()+"/problems/sample/" {
					t.Fatal("wrong judge origin or referer")
				}
				if r.Header.Get("Origin") != site.URL() || r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("unexpected POST headers: origin=%q accept=%q content-type=%q", r.Header.Get("Origin"), r.Header.Get("Accept"), r.Header.Get("Content-Type"))
				}
				var data map[string]string
				if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
					t.Fatal(err)
				}
				action, result := "interpret_solution", `{"interpret_id":"`+runID+`"}`
				if submit {
					action, result = "submit", `{"submission_id":12}`
					if _, ok := data["data_input"]; ok {
						t.Fatal("test input sent on submission")
					}
				} else if data["data_input"] != "1\n2" {
					t.Fatal("incorrect testcase sequence")
				}
				if r.Method != "POST" || r.URL.Path != "/problems/sample/"+action+"/" || data["typed_code"] != "sample code" {
					t.Fatal("wrong judge request")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(result)), Header: make(http.Header)}, nil
			})
			id, err := client.StartJudge(ctx, submit, p, "golang", "sample code", []string{"1", "2"}, nil)
			if err != nil || id == "" {
				t.Fatal(id, err)
			}
			if !submit && id != runID {
				t.Fatalf("expected run id %q, got %q", runID, id)
			}
		}

		// CheckJudge accepts real dotted runcode IDs and decodes Run array code_output.
		client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet || r.URL.Path != "/submissions/detail/"+runID+"/check/" {
				t.Fatalf("unexpected check request: %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Origin") != "" || r.Header.Get("Content-Type") != "" || r.Header.Get("Accept") != "application/json" {
				t.Fatalf("unexpected GET headers: origin=%q content-type=%q accept=%q", r.Header.Get("Origin"), r.Header.Get("Content-Type"), r.Header.Get("Accept"))
			}
			body := `{
				"state":"SUCCESS",
				"status_msg":"Accepted",
				"status_runtime":"4 ms",
				"compare_result":"11",
				"correct_answer":true,
				"code_answer":["[0,1]","[1,2]"],
				"expected_code_answer":["[0,1]","[1,2]"],
				"code_output":["dbg1\n",""],
				"std_output_list":["",""]
			}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		got, err := client.CheckJudge(ctx, runID, "sample", nil)
		if err != nil {
			t.Fatalf("CheckJudge failed on Run response: %v", err)
		}
		if got.State != "SUCCESS" || got.Compare != "11" || len(got.Outputs) != 2 || got.Outputs[0] != "[0,1]" {
			t.Fatalf("unexpected Run judgment: %+v", got)
		}
		if len(got.Stdout) != 2 || got.Stdout[0] != "dbg1\n" {
			t.Fatalf("expected code_output array fallback for stdout, got %+v", got.Stdout)
		}

		for _, badID := range []string{"", "../other", "a..b", ".leading", "trailing.", "a/b", strings.Repeat("a", 129)} {
			if _, err := client.CheckJudge(ctx, badID, "sample", nil); err == nil {
				t.Fatalf("invalid result identifier %q accepted", badID)
			}
		}
	}
}

func TestJudgmentUnmarshalVariantsAndStatusError(t *testing.T) {
	// Submit response with string code_output and short compile_error fallback.
	var submitResult Judgment
	submitJSON := `{
		"state":"SUCCESS",
		"status_msg":"Wrong Answer",
		"total_correct":10,
		"total_testcases":12,
		"last_testcase":"[1,2]\n3",
		"code_output":"[0,0]",
		"expected_output":"[0,1]",
		"compile_error":"syntax error on line 3"
	}`
	if err := json.Unmarshal([]byte(submitJSON), &submitResult); err != nil {
		t.Fatalf("unmarshal submit judgment: %v", err)
	}
	if submitResult.LastOutput != "[0,0]" || submitResult.LastExpected != "[0,1]" || submitResult.CompileError != "syntax error on line 3" {
		t.Fatalf("unexpected submit judgment: %+v", submitResult)
	}

	// Run response with non-empty std_output_list keeps std_output_list over code_output.
	var runResult Judgment
	runJSON := `{
		"state":"SUCCESS",
		"status_msg":"Runtime Error",
		"code_output":[],
		"std_output_list":["printed\n"],
		"runtime_error":"index out of range"
	}`
	if err := json.Unmarshal([]byte(runJSON), &runResult); err != nil {
		t.Fatalf("unmarshal run judgment: %v", err)
	}
	if len(runResult.Stdout) != 1 || runResult.Stdout[0] != "printed\n" || runResult.RuntimeError != "index out of range" {
		t.Fatalf("unexpected run judgment: %+v", runResult)
	}

	// HTTP non-429 errors return StatusError with site and code.
	client := New(CN)
	client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	_, err := client.StartJudge(context.Background(), false, Problem{ID: "1", Slug: "two-sum"}, "golang", "code", []string{"1"}, nil)
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusForbidden || err.Error() != "leetcode.cn returned HTTP 403" {
		t.Fatalf("expected StatusError 403, got %v", err)
	}

	// Unsafe slugs and empty IDs are rejected before issuing a request.
	called := false
	client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	if _, err := client.StartJudge(context.Background(), true, Problem{ID: "1", Slug: ".."}, "golang", "code", nil, nil); err == nil || called {
		t.Fatalf("expected traversal slug to fail before HTTP request, err=%v called=%v", err, called)
	}

	// Viewer must not retry on HTTP 429 rate limits, and Retry-After dates are capped at 24h.
	attempts := 0
	client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		attempts++
		h := make(http.Header)
		h.Set("Retry-After", "Wed, 21 Oct 2099 07:28:00 GMT")
		return &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader("")), Header: h}, nil
	})
	_, err = client.Viewer(context.Background(), nil)
	var rl *RateLimit
	if !errors.As(err, &rl) || attempts != 1 {
		t.Fatalf("expected single rate-limit attempt, attempts=%d err=%v", attempts, err)
	}
	if rl.RetryAt.Year() >= 2099 {
		t.Fatalf("Retry-After date was not capped: %v", rl.RetryAt)
	}
}
