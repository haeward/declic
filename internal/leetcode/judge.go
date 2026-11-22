package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Judgment struct {
	Compare           string   `json:"compare_result"`
	RuntimePercentile *float64 `json:"runtime_percentile"`
	MemoryPercentile  *float64 `json:"memory_percentile"`
	State             string   `json:"state"`
	Verdict           string   `json:"status_msg"`
	Runtime           string   `json:"status_runtime"`
	Memory            string   `json:"status_memory"`
	Passed            int      `json:"total_correct"`
	Total             int      `json:"total_testcases"`
	Correct           *bool    `json:"correct_answer"`
	Outputs           []string `json:"code_answer"`
	Expected          []string `json:"expected_code_answer"`
	Stdout            []string `json:"std_output_list"`
	LastInput         string   `json:"last_testcase"`
	LastOutput        string   `json:"code_output"`
	LastExpected      string   `json:"expected_output"`
	CompileError      string   `json:"full_compile_error"`
	RuntimeError      string   `json:"full_runtime_error"`
}

func (j *Judgment) UnmarshalJSON(data []byte) error {
	type judgmentAlias Judgment
	var wire struct {
		judgmentAlias
		CodeOutput        json.RawMessage `json:"code_output"`
		ShortCompileError string          `json:"compile_error"`
		ShortRuntimeError string          `json:"runtime_error"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*j = Judgment(wire.judgmentAlias)
	if j.CompileError == "" {
		j.CompileError = wire.ShortCompileError
	}
	if j.RuntimeError == "" {
		j.RuntimeError = wire.ShortRuntimeError
	}
	raw := bytes.TrimSpace(wire.CodeOutput)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	switch raw[0] {
	case '"':
		return json.Unmarshal(raw, &j.LastOutput)
	case '[':
		var lines []string
		if err := json.Unmarshal(raw, &lines); err != nil {
			return err
		}
		if !hasNonEmpty(j.Stdout) && hasNonEmpty(lines) {
			j.Stdout = lines
		}
	}
	return nil
}

func hasNonEmpty(values []string) bool {
	for _, v := range values {
		if v != "" {
			return true
		}
	}
	return false
}

var judgeID = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*$`)

func validJudgeID(id string) bool {
	return len(id) > 0 && len(id) <= 128 && judgeID.MatchString(id)
}

func (c *Client) judgeRequest(ctx context.Context, path, slug string, body any, cookies []*http.Cookie, result any) error {
	if c.Site != COM && c.Site != CN {
		return fmt.Errorf("invalid site")
	}
	var reader io.Reader
	method := http.MethodGet
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Site.URL()+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", c.Site.URL())
	}
	referer := c.Site.URL() + "/"
	if slug != "" {
		referer = c.Site.URL() + "/problems/" + url.PathEscape(slug) + "/"
	}
	req.Header.Set("Referer", referer)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		if cookie.Name == "csrftoken" {
			req.Header.Set("X-CSRFToken", cookie.Value)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return c.responseError(resp)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(result)
}
func (c *Client) StartJudge(ctx context.Context, submit bool, p Problem, language, code string, cases []string, cookies []*http.Cookie) (string, error) {
	action := "submit"
	payload := map[string]any{"question_id": p.ID, "lang": language, "typed_code": code}
	if !submit {
		action = "interpret_solution"
		payload["data_input"] = strings.Join(cases, "\n")
	}
	var response struct {
		Interpret  string      `json:"interpret_id"`
		Submission json.Number `json:"submission_id"`
	}
	err := c.judgeRequest(ctx, "/problems/"+url.PathEscape(p.Slug)+"/"+action+"/", p.Slug, payload, cookies, &response)
	if err != nil {
		return "", err
	}
	id := response.Interpret
	if submit {
		id = response.Submission.String()
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			return "", fmt.Errorf("missing submission identifier")
		}
	}
	if !validJudgeID(id) {
		return "", fmt.Errorf("missing run identifier")
	}
	return id, nil
}
func (c *Client) CheckJudge(ctx context.Context, id, slug string, cookies []*http.Cookie) (Judgment, error) {
	var result Judgment
	if !validJudgeID(id) {
		return result, fmt.Errorf("invalid result identifier")
	}
	err := c.judgeRequest(ctx, "/submissions/detail/"+id+"/check/", slug, nil, cookies, &result)
	return result, err
}
