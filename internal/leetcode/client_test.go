package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGuestRequestsAndErrors(t *testing.T) {
	for _, site := range []Site{CN, COM} {
		t.Run(string(site), func(t *testing.T) {
			client := New(site)
			client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				var request struct {
					Variables struct {
						Skip    int            `json:"skip"`
						Filters map[string]any `json:"filters"`
					} `json:"variables"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if _, exists := request.Variables.Filters["searchKeywords"]; exists {
					t.Fatal("empty search must omit searchKeywords")
				}
				if _, exists := request.Variables.Filters["status"]; exists {
					t.Fatal("guest request includes an account filter")
				}
				if request.Variables.Skip != 20 {
					t.Fatal("lost page offset")
				}
				if r.Header.Get("Cookie") != "" || r.Header.Get("X-CSRFToken") != "" {
					t.Fatal("guest request contains credentials")
				}
				if r.URL.Host != string(site) {
					t.Fatal("wrong site")
				}
				problem := `{"titleSlug":"sample","questionFrontendId":"1","title":"Sample"}`
				if site == CN {
					problem = `{"titleSlug":"sample","frontendQuestionId":"1","title":"Sample","titleCn":"样例"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"page":{"total":40,"questions":[` + problem + `]}}}`)), Header: make(http.Header)}, nil
			})
			page, err := client.List(context.Background(), "", 20)
			if err != nil || page.Total != 40 || len(page.Problems) != 1 {
				t.Fatalf("page=%+v err=%v", page, err)
			}
			if page.Problems[0].FrontendID != "1" || (site == CN && page.Problems[0].Title != "样例") {
				t.Fatalf("list fields not normalized: %+v", page.Problems[0])
			}
			client.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"errors":[{"message":"private details"}]}`))}, nil
			})
			if _, err = client.List(context.Background(), "", 0); err == nil || strings.Contains(err.Error(), "private details") {
				t.Fatal("GraphQL error handling")
			}
		})
	}
}

func TestViewerProfileEnrichmentAndPlaygroundCodes(t *testing.T) {
	for _, site := range []Site{COM, CN} {
		t.Run(string(site), func(t *testing.T) {
			client := New(site)
			client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				var req struct {
					Query string `json:"query"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				switch {
				case strings.Contains(req.Query, "userStatus"):
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"userStatus":{"isSignedIn":true,"username":"alice","isPremium":false}}}`)), Header: make(http.Header)}, nil
				case strings.Contains(req.Query, "matchedUser"):
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"matchedUser":{"profile":{"realName":"Alice Lin","userAvatar":"//assets.leetcode.com/users/alice.png"}}}}`)), Header: make(http.Header)}, nil
				case strings.Contains(req.Query, "userProfilePublicProfile"):
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"userProfilePublicProfile":{"profile":{"realName":"爱丽丝","userAvatar":"/aliyun/alice.png"}}}}`)), Header: make(http.Header)}, nil
				case strings.Contains(req.Query, "allPlaygroundCodes"):
					if r.Header.Get("Cookie") != "" {
						t.Fatal("playground query must not send cookies")
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"allPlaygroundCodes":[{"langSlug":"golang","code":"func twoSum() {}"}]}}`)), Header: make(http.Header)}, nil
				default:
					t.Fatalf("unexpected query: %s", req.Query)
					return nil, nil
				}
			})
			v, err := client.Viewer(context.Background(), []*http.Cookie{{Name: "LEETCODE_SESSION", Value: "tok"}})
			if err != nil {
				t.Fatal(err)
			}
			if v.RealName == "" || !strings.HasPrefix(v.Avatar, "https://") {
				t.Fatalf("viewer profile not enriched/normalized: %+v", v)
			}
			codes, err := client.PlaygroundCodes(context.Background(), "WTVGRyeD")
			if err != nil || len(codes) != 1 || codes[0].Language != "golang" {
				t.Fatalf("playground codes failed: %+v %v", codes, err)
			}
		})
	}
}

func TestCNVideoInfo(t *testing.T) {
	cn := New(CN)
	cn.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(req.Query, "videosVideoInfo") || req.Variables["uuid"] != "4547de8a-5963-4caf-9281-c22ee751ab12" {
			t.Fatalf("unexpected query/vars: %+v", req)
		}
		body := `{"data":{"videosVideoInfo":{"playAuth":"auth-token","canSee":true,"videoInfo":{"videoId":"vid123","coverUrl":"http://video.leetcode.cn/cover.jpg?auth_key=abc"}}}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	info, err := cn.CNVideoInfo(context.Background(), "4547de8a-5963-4caf-9281-c22ee751ab12", nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.VideoID != "vid123" || info.PlayAuth != "auth-token" || !info.CanSee || info.CoverURL != "https://video.leetcode.cn/cover.jpg?auth_key=abc" {
		t.Fatalf("unexpected CNVideoInfo: %+v", info)
	}
	if _, err := cn.CNVideoInfo(context.Background(), "../bad", nil); err == nil {
		t.Fatal("expected invalid UUID error")
	}
	if _, err := New(COM).CNVideoInfo(context.Background(), "4547de8a-5963-4caf-9281-c22ee751ab12", nil); err == nil {
		t.Fatal("expected CNVideoInfo to reject leetcode.com")
	}
}

// Opt-in integration checks use guest requests and never submit code.
func TestLiveGuest(t *testing.T) {
	if os.Getenv("DECLIC_LIVE_API") != "1" {
		t.Skip("set DECLIC_LIVE_API=1 for live guest API checks")
	}
	for _, site := range []Site{CN, COM} {
		client := New(site)
		t.Run(string(site)+"/catalog", func(t *testing.T) {
			rows, err := client.Catalog(context.Background())
			if err != nil || len(rows) == 0 {
				t.Fatalf("catalog unavailable: %v", err)
			}
		})
		t.Run(string(site)+"/daily", func(t *testing.T) {
			daily, err := client.Daily(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if daily.Date == "" || daily.Problem.Slug == "" || daily.Problem.FrontendID == "" {
				t.Fatalf("invalid daily challenge: %+v", daily)
			}
			t.Logf("date=%s problem=%s", daily.Date, daily.Problem.Slug)
		})
		for _, skip := range []int{0, 20} {
			t.Run(fmt.Sprintf("%s/page%d", site, skip), func(t *testing.T) {
				page, err := client.List(context.Background(), "", skip)
				if err != nil {
					t.Fatal(err)
				}
				if len(page.Problems) == 0 {
					t.Fatal("empty page")
				}
				for _, problem := range page.Problems {
					if problem.FrontendID == "" || problem.Title == "" || problem.Slug == "" {
						t.Fatal("missing list identity or display fields")
					}
				}
				t.Logf("items=%d total=%d first=%s", len(page.Problems), page.Total, page.Problems[0].Slug)
			})
		}
		t.Run(string(site)+"/search", func(t *testing.T) {
			page, err := client.List(context.Background(), "two sum", 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Problems) == 0 {
				t.Fatal("empty search")
			}
			t.Logf("items=%d", len(page.Problems))
		})
		t.Run(string(site)+"/empty", func(t *testing.T) {
			// Search is fuzzy, particularly on CN. A page beyond the known
			// catalog is a reproducible empty response on either site.
			first, err := client.List(context.Background(), "", 0)
			if err != nil || first.Total <= 0 {
				t.Fatalf("could not determine catalog size: total=%d err=%v", first.Total, err)
			}
			page, err := client.List(context.Background(), "", first.Total+20)
			if err != nil || len(page.Problems) != 0 {
				t.Fatalf("expected successful empty result: total=%d items=%d err=%v", page.Total, len(page.Problems), err)
			}
			t.Logf("past-end page empty; reported total=%d", page.Total)
		})
		t.Run(string(site)+"/member", func(t *testing.T) {
			page, err := client.List(context.Background(), "Meeting Rooms", 0)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, problem := range page.Problems {
				found = found || problem.Slug == "meeting-rooms"
			}
			if !found {
				t.Fatal("member question missing from the guest list")
			}
			p, err := client.Detail(context.Background(), "meeting-rooms")
			if err != nil || !p.Paid {
				t.Fatalf("expected member metadata: paid=%t err=%v", p.Paid, err)
			}
			if p.Content != "" || p.TranslatedContent != "" || len(p.Snippets) != 0 {
				t.Fatal("unexpected anonymous member content")
			}
			t.Log("member metadata visible; no content or templates")
		})
		t.Run(string(site)+"/detail", func(t *testing.T) {
			p, err := client.Detail(context.Background(), "two-sum")
			if err != nil {
				t.Fatal(err)
			}
			if p.Content == "" || len(p.Snippets) == 0 {
				t.Fatal("missing content or templates")
			}
			t.Logf("id=%s templates=%d", p.ID, len(p.Snippets))
		})
	}
}

func TestLiveGuestArticles(t *testing.T) {
	if os.Getenv("DECLIC_LIVE_API") != "1" {
		t.Skip("set DECLIC_LIVE_API=1 for live guest API checks")
	}
	for _, site := range []Site{CN, COM} {
		c := New(site)
		ctx := context.Background()
		t.Run(string(site)+"/editorial", func(t *testing.T) {
			_, err := c.EditorialForViewer(ctx, "two-sum", nil)
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run(string(site)+"/solutions", func(t *testing.T) {
			rows, err := c.Solutions(ctx, "two-sum", "HOT", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				t.Fatal("missing solutions")
			}
			_, err = c.Solution(ctx, rows[0].Slug)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
