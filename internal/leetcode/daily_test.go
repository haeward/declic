package leetcode

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDailySiteMappingAndGuestBoundary(t *testing.T) {
	for _, site := range []Site{CN, COM} {
		t.Run(string(site), func(t *testing.T) {
			client := New(site)
			client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != string(site) || r.Header.Get("Cookie") != "" || r.Header.Get("X-CSRFToken") != "" {
					t.Fatal("daily request crossed the guest/site boundary")
				}
				query, _ := io.ReadAll(r.Body)
				body := `{"data":{"activeDailyCodingChallengeQuestion":{"date":"2026-11-12","question":{"questionFrontendId":"1","title":"Sample","titleSlug":"sample"}}}}`
				if site == CN {
					if !strings.Contains(string(query), "titleCn:translatedTitle") || strings.Contains(string(query), "frontendQuestionId") {
						t.Fatal("CN daily must use QuestionNode fields, not catalog fields")
					}
					body = `{"data":{"todayRecord":[{"date":"2026-11-12","question":{"questionFrontendId":"1","title":"Sample","titleCn":"样例","titleSlug":"sample"}}]}}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			daily, err := client.Daily(context.Background())
			if err != nil || daily.Problem.FrontendID != "1" || (site == CN && daily.Problem.Title != "样例") {
				t.Fatalf("daily=%+v err=%v", daily, err)
			}
		})
	}
}

func TestDailyRejectsUnavailableData(t *testing.T) {
	for _, body := range []string{`{"data":{"todayRecord":[]}}`, `{"data":{"todayRecord":[{"date":"invalid","question":{"title":"Sample","titleSlug":"sample"}}]}}`, `{"data":{"todayRecord":[{"date":"2026-11-12","question":null}]}}`} {
		c := New(CN)
		c.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		if _, err := c.Daily(context.Background()); err == nil {
			t.Fatal("accepted missing daily challenge")
		}
	}
}
