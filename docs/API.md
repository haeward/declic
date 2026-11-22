# LeetCode API integration

This document describes the website interfaces used by Declic. It is not an official LeetCode API specification or a public Declic SDK. The implementation in [internal/leetcode/](../internal/leetcode) (`client.go`, `judge.go`, `resources.go`, `status.go`) is the source for the requests below; server behavior can change independently.

[Leetgo](https://github.com/j178/leetgo) is an implementation reference for LeetCode integration. Features in a reference client do not establish support in Declic.

## Transport

| Site | Origin | GraphQL endpoint |
| --- | --- | --- |
| CN | `https://leetcode.cn` | `POST /graphql` |
| COM | `https://leetcode.com` | `POST /graphql` |

Resolve all requests against the selected site's origin. GraphQL requests send JSON containing `query` and `variables`, with `Content-Type: application/json`, `Accept: application/json`, and a same-site root `Referer`. REST requests (Online Judge and account solve-status) send `Accept: application/json`, plus `Content-Type: application/json`, `Origin: <site origin>`, and problem-scoped `Referer` (`/problems/<slug>/`) on `POST`.

The client has a 20-second HTTP timeout, accepts a `context.Context`, limits response decoding to 4 MiB for GraphQL and 2 MiB for REST endpoints, and refuses redirects. It has no cookie jar and performs no automatic retries. Public list, catalog, daily challenge, detail, article, and Playground requests pass no credentials. Account requests receive explicit site cookies (`LEETCODE_SESSION`, `csrftoken`, and `cf_clearance`, read from the selected site's MyGo session with an 8-second timeout); when `csrftoken` is present, its value is also sent as `X-CSRFToken`. Credential policy is defined in [SECURITY.md](../SECURITY.md).

## Catalog and search

Both sites use `categorySlug: "algorithms"`, a `limit`, a `skip` offset, and a `QuestionListFilterInput` object. `List` requests 20 rows for fast initial rendering (sending nonempty search text as `filters.searchKeywords`), while `Catalog` fetches the full public index in pages of 1000 rows using the actual returned length and a 300 ms delay between pages. The desktop stores public metadata in a per-site cache (24-hour TTL) and performs search, tag matching, sorting, and pagination locally on the index.

| Selection | CN | COM |
| --- | --- | --- |
| Root field | `problemsetQuestionList` | `questionList` |
| Total count | `total` | `totalNum` |
| Rows | `questions` | `data` |
| Display number | `frontendQuestionId` | `questionFrontendId` |
| Title | `title`, with `titleCn` preferred when present | `title` |
| Routing key | `titleSlug` | `titleSlug` |
| Metadata | `difficulty`, `acRate`, `topicTags { name slug }`, `paidOnly` (aliased to `isPaidOnly`) | `difficulty`, `acRate`, `topicTags { name slug }`, `isPaidOnly` |

The queries alias the root to `page`; COM also aliases `totalNum` to `total` and `data` to `questions`. These aliases are Declic's response mapping, not additional server fields. `Problem.Normalize()` trims stray whitespace from titles and IDs and reconciles display fields. An existing list root with zero rows is a successful empty result.

## Daily Challenge

CN reads `todayRecord { date question { questionId questionFrontendId title titleCn:translatedTitle titleSlug difficulty topicTags { name slug } acRate } }`; `translatedTitle` is aliased to `titleCn` for the shared display mapping. This is a `QuestionNode`, whose title and display-number fields differ from CN catalog rows. COM reads `activeDailyCodingChallengeQuestion { date question { questionId questionFrontendId title titleSlug difficulty topicTags { name slug } acRate isPaidOnly } }`.

The client validates the date (`YYYY-MM-DD`) and problem routing fields. Daily requests use no cookies. The UI displays the source date, uses the site's day boundary (`UTC` for COM, `UTC+8` for CN) when deciding whether to refresh the card, silently prefetches non-paid daily problem details into cache, and shows a separate retry state on failure. Opening a challenge uses the ordinary detail/draft flow; daily metadata does not grant access to restricted content.

## Problem detail

Both sites use `question(titleSlug: $slug)` with `$slug: String!` and read `data.question`.

| Selected fields | Meaning |
| --- | --- |
| `questionId` | Internal problem identifier within the selected site |
| `questionFrontendId` | Display number; kept as a string |
| `title`, `titleSlug` | Title and URL slug |
| `content`, `translatedContent` | HTML descriptions; Declic prefers a nonempty translation unless English is toggled |
| `difficulty` | Difficulty label, omitted visually when unavailable |
| `isPaidOnly` | Restricted-problem marker |
| `exampleTestcaseList`, `metaData` | Default sample testcases and parameter metadata for the Testcase editor |
| `hints`, `similarQuestions`, `topicTags { name slug }` | Collapsible hints, related problems JSON, and topic chips |
| `codeSnippets { langSlug code }` | Language identifiers and starter templates |

Detail requests start anonymously. For a paid problem, the desktop verifies the selected site's viewer and Premium entitlement before requesting the detail with session cookies (`DetailForViewer`). Access still requires a returned statement and templates. Non-paid details are cached on disk for 7 days; paid details remain strictly in memory.

## Session identity and solve status

The desktop opens `/accounts/login/` on the selected site for the user to sign in, then validates that site's session with `userStatus` (reading `isSignedIn`, `username`, `isPremium`, `avatar`, and on CN `realName` and `userSlug`, with a fallback query of `isSignedIn username isPremium`):

```graphql
query {
  userStatus {
    isSignedIn
    username
    isPremium
    avatar
  }
}
```

When signed in and profile fields are incomplete, the client enriches the viewer with public profile data (`matchedUser` on COM, `userProfilePublicProfile` on CN) to obtain display name and an HTTPS-normalized avatar URL.

The desktop requires both a signed-in result and a nonempty username before selecting account data. A cookie's presence alone does not establish identity. While the login window is open, Declic automatically checks session cookies on navigation and periodic intervals (with a manual retry fallback on error), and caches verified credentials in memory for 5 minutes (invalidated on sign-out, site switch, expiry, or 401/403 judge errors). At startup, if `Restore session on launch` is enabled and a `LEETCODE_SESSION` cookie exists for the selected site, Declic verifies that session via `userStatus` before selecting account data (guests with no session cookie make no request); it never auto-restores identity across site switches.

The account solve-status request reads the selected site's `/api/problems/algorithms/` REST endpoint (`stat_status_pairs`) with verified cookies and keeps solved flags strictly in memory.

## Workspace resources

- **Editorial**: Queries `question(titleSlug: $slug) { solution { title content paidOnly canSeeDetail } }` (`paidOnly` omitted on CN). Public visible editorials are cached on disk for 7 days; paid editorials stay in memory.
- **Community solutions**: Lists 20 rows per page via `questionSolutionArticles` (CN) or `ugcArticleSolutionArticles` (COM), mapping order values (`HOT`, `NEWEST` → `NEWEST_TO_OLDEST` on CN / `MOST_RECENT` on COM, `MOST_VOTES` → `MOST_UPVOTE` on CN; `upvoteCount` queried on CN only) and optional language tag filters. Individual articles load via `solutionArticle` (CN) or `ugcArticleSolutionArticle` (COM).
- **Embedded Playground snippets**: Public code tabs embedded in Editorial or Solutions articles fetch snippets anonymously via `allPlaygroundCodes(uuid: $uuid) { code langSlug }`.
- **Submissions history**: Authenticated queries use `submissionList(questionSlug: $slug, offset: $skip, limit: 20)` and fetch code via `submissionDetail` (CN, `$id: ID!`) or `submissionDetails` (COM, `$id: Int!`). Private submissions are never written to disk caches.

## Online Judge (Run and Submit)

Run and Submit require a verified viewer for the selected site and never retry automatically:

| Operation | Endpoint | Payload / Behavior |
| --- | --- | --- |
| Run | `POST /problems/<slug>/interpret_solution/` | Sends `question_id`, `lang`, `typed_code`, and newline-joined `data_input`; returns `interpret_id` |
| Submit | `POST /problems/<slug>/submit/` | Sends `question_id`, `lang`, and `typed_code`; returns numeric `submission_id`; enforces a 15-second local cooldown after dispatch |
| Poll result | `GET /submissions/detail/<id>/check/` | Validates `<id>` format; polls every 1.5 seconds up to a 60-second local timeout until `state == "SUCCESS"` or terminal error |

`Judgment` normalizes site response variants: `code_output` may arrive as a string (single failed submission output) or string array (stdout lines when `std_output_list` is empty), and compile/runtime errors fall back from `full_compile_error` / `full_runtime_error` to `compile_error` / `runtime_error`.

## Errors and verification

The client rejects non-200 HTTP responses, invalid JSON, nonempty GraphQL `errors`, and missing or null `data`. Individual operations also reject a missing list root, a missing viewer, an invalid judge identifier, or a detail result without `questionId`. Raw GraphQL error contents are not included in its error message.

In [internal/leetcode/status.go](../internal/leetcode/status.go), non-200 HTTP responses return `*StatusError` (recording site and status code), except `HTTP 429` which returns `*RateLimit` (parsing `Retry-After` as delta-seconds up to 24 hours or an HTTP-date, defaulting to 1 minute). HTTP failures, GraphQL failures, and empty results are distinct. The client does not classify every authentication, challenge, or site-specific failure; in particular, a 403 alone does not identify the underlying cause.

Offline tests cover request construction, normalization, error classification, and judge response variants using synthetic HTTP transports. Opt-in live checks (`TestLiveGuest` and `TestLiveGuestArticles`) cover public reads on both sites; commands and their scope are in [CONTRIBUTING.md](../CONTRIBUTING.md). Authenticated workflows (login, judging, account history, and Premium access) require manual verification with an account. Live observations apply to the site, environment, and time tested.
