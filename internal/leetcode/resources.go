package leetcode

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Article struct {
	UUID    string `json:"uuid"`
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	Content string `json:"content"`
	Paid    bool   `json:"paidOnly"`
	Visible bool   `json:"canSeeDetail"`
	Views   int    `json:"hitCount"`
	Votes   *int   `json:"upvoteCount"`
	Author  struct {
		Name string `json:"userName"`
	} `json:"author"`
}

func (c *Client) Editorial(ctx context.Context, slug string) (*Article, error) {
	return c.EditorialForViewer(ctx, slug, nil)
}
func (c *Client) EditorialForViewer(ctx context.Context, slug string, cookies []*http.Cookie) (*Article, error) {
	var r struct{ Question struct{ Solution *Article } }
	query := `query($slug:String!){question(titleSlug:$slug){solution{title content paidOnly canSeeDetail}}}`
	if c.Site == CN {
		query = strings.Replace(query, "paidOnly ", "", 1)
	}
	err := c.query(ctx, query, map[string]any{"slug": slug}, cookies, &r)
	return r.Question.Solution, err
}
func (c *Client) Solutions(ctx context.Context, slug, order, lang string, skip int) ([]Article, error) {
	tags := []string{}
	if lang != "" {
		tags = append(tags, lang)
	}
	if order == "NEWEST" {
		order = "MOST_RECENT"
		if c.Site == CN {
			order = "NEWEST_TO_OLDEST"
		}
	} else if order == "MOST_VOTES" && c.Site == CN {
		order = "MOST_UPVOTE"
	}
	vars := map[string]any{"slug": slug, "order": order, "tags": tags, "skip": skip}
	var r struct {
		List struct{ Edges []struct{ Node Article } }
	}
	query := `query($slug:String!,$order:ArticleOrderByEnum,$tags:[String!],$skip:Int!){list:ugcArticleSolutionArticles(questionSlug:$slug,orderBy:$order,tagSlugs:$tags,first:20,skip:$skip){edges{node{uuid slug title hitCount upvoteCount author{userName}}}}}`
	if c.Site == CN {
		query = strings.NewReplacer("ugcArticleSolutionArticles", "questionSolutionArticles", "ArticleOrderByEnum", "SolutionArticleOrderBy", "author{userName}", "author{userName:username}").Replace(query)
	} else {
		query = strings.Replace(query, "upvoteCount ", "", 1)
	}
	err := c.query(ctx, query, vars, nil, &r)
	var rows []Article
	for _, e := range r.List.Edges {
		rows = append(rows, e.Node)
	}
	return rows, err
}
func (c *Client) Solution(ctx context.Context, slug string) (*Article, error) {
	var r struct{ Article *Article }
	query := `query($slug:String!){article:ugcArticleSolutionArticle(slug:$slug){title content}}`
	if c.Site == CN {
		query = strings.Replace(query, "ugcArticleSolutionArticle", "solutionArticle", 1)
	}
	err := c.query(ctx, query, map[string]any{"slug": slug}, nil, &r)
	return r.Article, err
}

type Submission struct {
	ID        string `json:"id"`
	Status    string `json:"statusDisplay"`
	Language  string `json:"lang"`
	Runtime   string `json:"runtime"`
	Memory    string `json:"memory"`
	Timestamp string `json:"timestamp"`
	Code      string `json:"code"`
}

func (c *Client) Submissions(ctx context.Context, slug string, skip int, cookies []*http.Cookie) ([]Submission, bool, error) {
	var r struct {
		List struct {
			Rows []Submission `json:"submissions"`
			More bool         `json:"hasNext"`
		}
	}
	query := `query($slug:String!,$skip:Int!){list:submissionList(questionSlug:$slug,offset:$skip,limit:20){hasNext submissions{id statusDisplay lang runtime memory timestamp}}}`
	err := c.query(ctx, query, map[string]any{"slug": slug, "skip": skip}, cookies, &r)
	return r.List.Rows, r.List.More, err
}
func (c *Client) SubmissionCode(ctx context.Context, id int, cookies []*http.Cookie) (string, error) {
	var r struct{ Details struct{ Code string } }
	query := `query($id:Int!){details:submissionDetails(submissionId:$id){code}}`
	vars := map[string]any{"id": id}
	if c.Site == CN {
		query = `query($id:ID!){details:submissionDetail(submissionId:$id){code}}`
		vars["id"] = strconv.Itoa(id)
	}
	err := c.query(ctx, query, vars, cookies, &r)
	return r.Details.Code, err
}

// PlaygroundCodes fetches public code snippets for an embedded LeetCode playground.
func (c *Client) PlaygroundCodes(ctx context.Context, uuid string) ([]Snippet, error) {
	var r struct {
		Codes []Snippet `json:"allPlaygroundCodes"`
	}
	query := `query($uuid:String!){allPlaygroundCodes(uuid:$uuid){code langSlug}}`
	if err := c.query(ctx, query, map[string]any{"uuid": uuid}, nil, &r); err != nil {
		return nil, err
	}
	return r.Codes, nil
}

// Solved reads the signed-in viewer's public problem metadata and solve flags.
func (c *Client) Solved(ctx context.Context, cookies []*http.Cookie) (map[string]bool, error) {
	var r struct {
		Rows []struct {
			Status string
			Stat   struct {
				Slug string `json:"question__title_slug"`
			}
		} `json:"stat_status_pairs"`
	}
	err := c.judgeRequest(ctx, "/api/problems/algorithms/", "", nil, cookies, &r)
	if err != nil {
		return nil, err
	}
	if r.Rows == nil {
		return nil, fmt.Errorf("missing problem statuses")
	}
	solved := map[string]bool{}
	for _, row := range r.Rows {
		if row.Status == "ac" {
			solved[row.Stat.Slug] = true
		}
	}
	return solved, nil
}
