package main

import (
	"strings"
	"testing"

	"github.com/haeward/declic/internal/leetcode"
)

func TestHintsSeparateAndSimilarQuestionsMetadata(t *testing.T) {
	a := &app{
		site:            leetcode.COM,
		descriptionSize: 15,
		catalog: []leetcode.Problem{
			{FrontendID: "15", Slug: "3sum", Title: "3Sum", Difficulty: "Medium", Paid: false},
			{FrontendID: "170", Slug: "two-sum-iii", Title: "Two Sum III", Difficulty: "Easy", Paid: true},
		},
		problem: leetcode.Problem{
			Hints:   []string{"Try a hash map.", "Watch out for <code>duplicates</code>."},
			Similar: `[{"title":"3Sum","titleSlug":"3sum","difficulty":"Medium","isPaidOnly":false},{"title":"Two Sum III","titleSlug":"two-sum-iii","difficulty":"Easy","isPaidOnly":true}]`,
		},
	}
	got := a.statementHTML("<p>Statement</p>")
	for _, want := range []string{
		"Hint 1</span></summary>",
		"Hint 2</span></summary>",
		"<code>duplicates</code>",
		"Similar Questions (2)",
		`href="https://declic.local/problem/3sum"`,
		">15.<",
		">170.<",
		"Premium",
		"Medium",
		"Easy",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("statementHTML missing %q", want)
		}
	}
}

func TestSlidesLoginURLAndJudgeDiffHelpers(t *testing.T) {
	urls := resolveSlideManifestURLs(leetcode.COM, "two-sum", "../Documents/1_fix_1.json:1000,562")
	if len(urls) == 0 || !strings.HasSuffix(urls[0], "/Documents/1_fix_1.json") {
		t.Fatalf("slide manifest URL resolution failed: %+v", urls)
	}
	for _, bad := range []string{"//evil.com/manifest.json", "HTTPS://evil.com/manifest.json", "http://leetcode.com/manifest.json"} {
		if got := resolveSlideManifestURLs(leetcode.COM, "two-sum", bad); len(got) != 0 {
			t.Fatalf("resolveSlideManifestURLs accepted untrusted spec %q: %+v", bad, got)
		}
	}
	if !isPostLoginSiteURL(leetcode.COM, "https://leetcode.com/problemset/?next=/accounts/login") ||
		isPostLoginSiteURL(leetcode.COM, "https://leetcode.com.evil.com/problemset/") ||
		isPostLoginSiteURL(leetcode.COM, "https://leetcode.com/accounts/login/") {
		t.Fatal("isPostLoginSiteURL host/path validation failed")
	}
	frames := parseSlideManifestFrames("https://leetcode.com/problems/two-sum/Documents/1_fix_1.json", []byte(`{"timeline":[{"image":"../Figures/1/1_1.png"},{"image":"../Figures/1/1_2.png"}]}`))
	if len(frames) != 2 || frames[0] != "https://leetcode.com/problems/two-sum/Figures/1/1_1.png" {
		t.Fatalf("slide frame parsing failed: %+v", frames)
	}

	p, ma, mb, s := diffCommonPrefixSuffix("[0,1,2,3]", "[0,1,9,3]")
	if p != "[0,1," || ma != "2" || mb != "9" || s != ",3]" {
		t.Fatalf("diffCommonPrefixSuffix mismatch: %q %q %q %q", p, ma, mb, s)
	}
	if _, trunc := truncateResultText(strings.Repeat("x", 300)); !trunc {
		t.Fatal("long result text should truncate")
	}
}
