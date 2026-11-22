package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Site string

const (
	CN  Site = "leetcode.cn"
	COM Site = "leetcode.com"
)

func (s Site) URL() string { return "https://" + string(s) }

type Topic struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type Problem struct {
	EnglishTitle      string    `json:"englishTitle,omitempty"`
	Topics            []Topic   `json:"topicTags"`
	Examples          []string  `json:"exampleTestcaseList"`
	Hints             []string  `json:"hints"`
	Similar           string    `json:"similarQuestions"`
	Metadata          string    `json:"metaData"`
	ID                string    `json:"questionId"`
	FrontendID        string    `json:"questionFrontendId"`
	ListFrontendID    string    `json:"frontendQuestionId"`
	TranslatedTitle   string    `json:"titleCn"`
	Slug              string    `json:"titleSlug"`
	Title             string    `json:"title"`
	Content           string    `json:"content"`
	TranslatedContent string    `json:"translatedContent"`
	Paid              bool      `json:"isPaidOnly"`
	Difficulty        string    `json:"difficulty"`
	Acceptance        *float64  `json:"acRate"`
	Snippets          []Snippet `json:"codeSnippets"`
}

// Normalize trims stray whitespace returned by LeetCode endpoints and reconciles display fields.
func (p *Problem) Normalize() {
	p.ID = strings.TrimSpace(p.ID)
	p.FrontendID = strings.TrimSpace(p.FrontendID)
	p.ListFrontendID = strings.TrimSpace(p.ListFrontendID)
	p.Slug = strings.TrimSpace(p.Slug)
	p.Title = strings.TrimSpace(p.Title)
	p.TranslatedTitle = strings.TrimSpace(p.TranslatedTitle)
	p.EnglishTitle = strings.TrimSpace(p.EnglishTitle)
	if p.FrontendID == "" {
		p.FrontendID = p.ListFrontendID
	}
	if p.TranslatedTitle != "" {
		if p.EnglishTitle == "" && p.Title != p.TranslatedTitle {
			p.EnglishTitle = p.Title
		}
		p.Title = p.TranslatedTitle
	}
}

type Snippet struct {
	Language string `json:"langSlug"`
	Code     string `json:"code"`
}

type Page struct {
	Total    int       `json:"total"`
	Problems []Problem `json:"questions"`
}

type Viewer struct {
	Premium  bool   `json:"isPremium"`
	SignedIn bool   `json:"isSignedIn"`
	Username string `json:"username"`
	UserSlug string `json:"userSlug,omitempty"`
	RealName string `json:"realName,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

// Client has no cookie jar. Only account operations accept explicit credentials.
type Client struct {
	Site Site
	HTTP *http.Client
}

func New(site Site) *Client {
	return &Client{Site: site, HTTP: &http.Client{
		Timeout:       20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *Client) query(ctx context.Context, query string, variables any, cookies []*http.Cookie, result any) error {
	if c.Site != CN && c.Site != COM {
		return fmt.Errorf("unsupported site")
	}
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Site.URL()+"/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", c.Site.URL()+"/")
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
	if resp.StatusCode != http.StatusOK {
		return c.responseError(resp)
	}
	var envelope struct {
		Data   json.RawMessage   `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("invalid API response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("GraphQL returned %d error(s)", len(envelope.Errors))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("API returned no data")
	}
	return json.Unmarshal(envelope.Data, result)
}

func (c *Client) List(ctx context.Context, search string, skip int) (Page, error) {
	return c.list(ctx, search, skip, 20)
}

func (c *Client) Catalog(ctx context.Context) ([]Problem, error) {
	var rows []Problem
	for {
		p, err := c.list(ctx, "", len(rows), 1000)
		if err != nil {
			return nil, err
		}
		rows = append(rows, p.Problems...)
		if len(rows) >= p.Total {
			return rows, nil
		}
		if len(p.Problems) == 0 {
			return nil, fmt.Errorf("site returned an incomplete catalog")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}

}
func (c *Client) list(ctx context.Context, search string, skip, limit int) (Page, error) {
	query := `query($limit:Int,$skip:Int,$filters:QuestionListFilterInput){ page:problemsetQuestionList(categorySlug:"algorithms",limit:$limit,skip:$skip,filters:$filters){total questions{title titleCn titleSlug frontendQuestionId topicTags{name slug} difficulty acRate isPaidOnly:paidOnly}}}`
	if c.Site == COM {
		query = `query($limit:Int,$skip:Int,$filters:QuestionListFilterInput){ page:questionList(categorySlug:"algorithms",limit:$limit,skip:$skip,filters:$filters){total:totalNum questions:data{title titleSlug questionFrontendId topicTags{name slug} difficulty acRate isPaidOnly}}}`
	}
	var result struct {
		Page *Page `json:"page"`
	}
	filters := map[string]any{}
	if search != "" {
		filters["searchKeywords"] = search
	}
	err := c.query(ctx, query, map[string]any{"skip": skip, "limit": limit, "filters": filters}, nil, &result)
	if err != nil {
		return Page{}, err
	}
	if result.Page == nil {
		return Page{}, fmt.Errorf("API returned no problem list")
	}
	for i := range result.Page.Problems {
		result.Page.Problems[i].Normalize()
	}
	return *result.Page, nil
}

func (c *Client) Detail(ctx context.Context, slug string) (Problem, error) {
	return c.DetailForViewer(ctx, slug, nil)
}
func (c *Client) DetailForViewer(ctx context.Context, slug string, cookies []*http.Cookie) (Problem, error) {
	query := `query($slug:String!){question(titleSlug:$slug){questionId questionFrontendId title titleSlug content translatedContent difficulty isPaidOnly exampleTestcaseList metaData similarQuestions hints topicTags{name slug} codeSnippets{langSlug code}}}`
	var result struct {
		Problem *Problem `json:"question"`
	}
	err := c.query(ctx, query, map[string]any{"slug": slug}, cookies, &result)
	if err != nil {
		return Problem{}, err
	}
	if result.Problem == nil || result.Problem.ID == "" {
		return Problem{}, fmt.Errorf("problem unavailable")
	}
	result.Problem.Normalize()
	return *result.Problem, nil
}

func normalizeSiteAssetURL(site Site, raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "//") {
		s = "https:" + s
	} else if strings.HasPrefix(s, "/") {
		s = site.URL() + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.User != nil {
		return ""
	}
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	if u.Scheme != "https" {
		return ""
	}
	return u.String()
}

func shouldFallbackViewerQuery(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	var rl *RateLimit
	if errors.As(err, &rl) {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return se.StatusCode == http.StatusBadRequest
	}
	return strings.HasPrefix(err.Error(), "GraphQL returned ")
}

func (c *Client) Viewer(ctx context.Context, cookies []*http.Cookie) (Viewer, error) {
	var result struct {
		Viewer *Viewer `json:"userStatus"`
	}
	query := `{userStatus{isSignedIn username isPremium avatar}}`
	if c.Site == CN {
		query = `{userStatus{isSignedIn username isPremium avatar realName userSlug}}`
	}
	err := c.query(ctx, query, nil, cookies, &result)
	if shouldFallbackViewerQuery(ctx, err) {
		result.Viewer = nil
		err = c.query(ctx, `{userStatus{isSignedIn username isPremium}}`, nil, cookies, &result)
	}
	if err != nil {
		return Viewer{}, err
	}
	if result.Viewer == nil {
		return Viewer{}, fmt.Errorf("missing user status")
	}
	v := *result.Viewer
	if v.SignedIn && v.Username != "" && (v.RealName == "" || v.Avatar == "") {
		c.enrichViewerProfile(ctx, cookies, &v)
	}
	v.Avatar = normalizeSiteAssetURL(c.Site, v.Avatar)
	return v, nil
}

func (c *Client) enrichViewerProfile(ctx context.Context, cookies []*http.Cookie, v *Viewer) {
	type profileFields struct {
		RealName   string `json:"realName"`
		UserAvatar string `json:"userAvatar"`
	}
	apply := func(p profileFields) {
		if v.RealName == "" {
			v.RealName = strings.TrimSpace(p.RealName)
		}
		if v.Avatar == "" {
			v.Avatar = strings.TrimSpace(p.UserAvatar)
		}
	}
	if c.Site == CN {
		slug := v.UserSlug
		if slug == "" {
			slug = v.Username
		}
		var r struct {
			Public *struct {
				Profile profileFields `json:"profile"`
			} `json:"userProfilePublicProfile"`
		}
		if err := c.query(ctx, `query($userSlug:String!){userProfilePublicProfile(userSlug:$userSlug){profile{realName userAvatar}}}`, map[string]any{"userSlug": slug}, cookies, &r); err == nil && r.Public != nil {
			apply(r.Public.Profile)
		}
		return
	}
	var r struct {
		Matched *struct {
			Profile profileFields `json:"profile"`
		} `json:"matchedUser"`
	}
	if err := c.query(ctx, `query($username:String!){matchedUser(username:$username){profile{realName userAvatar}}}`, map[string]any{"username": v.Username}, cookies, &r); err == nil && r.Matched != nil {
		apply(r.Matched.Profile)
	}
}

// Daily is public site data. It never uses the signed-in browser session.
type Daily struct {
	Date    string  `json:"date"`
	Problem Problem `json:"question"`
}

func (c *Client) Daily(ctx context.Context) (Daily, error) {
	var daily Daily
	if c.Site == CN {
		var result struct {
			Records []Daily `json:"todayRecord"`
		}
		err := c.query(ctx, `{todayRecord{date question{questionId questionFrontendId title titleCn:translatedTitle titleSlug difficulty topicTags{name slug} acRate}}}`, nil, nil, &result)
		if err != nil {
			return Daily{}, err
		}
		if len(result.Records) != 1 {
			return Daily{}, fmt.Errorf("daily challenge unavailable")
		}
		daily = result.Records[0]
	} else {
		var result struct {
			Daily *Daily `json:"activeDailyCodingChallengeQuestion"`
		}
		err := c.query(ctx, `{activeDailyCodingChallengeQuestion{date question{questionId questionFrontendId title titleSlug difficulty topicTags{name slug} acRate isPaidOnly}}}`, nil, nil, &result)
		if err != nil {
			return Daily{}, err
		}
		if result.Daily == nil {
			return Daily{}, fmt.Errorf("daily challenge unavailable")
		}
		daily = *result.Daily
	}
	daily.Problem.Normalize()
	if _, err := time.Parse("2006-01-02", daily.Date); err != nil || daily.Problem.Slug == "" || daily.Problem.Title == "" {
		return Daily{}, fmt.Errorf("invalid daily challenge")
	}
	return daily, nil
}
