# Contribute to Declic

Bug reports, documentation improvements, testing, accessibility feedback, and code contributions are welcome. Please follow the [Code of Conduct](CODE_OF_CONDUCT.md); report vulnerabilities through [SECURITY.md](SECURITY.md).

## Before you start

Search existing [issues](https://github.com/haeward/declic/issues) and [pull requests](https://github.com/haeward/declic/pulls). Leave a comment before taking on an existing issue to coordinate with others.

Discuss new features, dependencies, and substantial architectural changes in an issue before implementation. Small bug fixes and documentation corrections can go directly to a pull request.

Bug reports should include the revision, operating system, reproduction steps, and expected versus actual behavior. For site-specific problems, name the LeetCode site and whether you were signed in. Remove credentials and personal data from logs and screenshots.

## Setup

Fork the repository, clone your fork, and create a branch from `main`. Use the Go and MyGo versions pinned in [go.mod](go.mod), and run commands from the repository root:

```sh
go tool mygo doctor   # Check platform prerequisites
go tool mygo dev      # Build and run the app
```

Use `go tool mygo` to run the pinned CLI. No separate JavaScript frontend toolchain is required. See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for the code structure, [API.md](docs/API.md) for LeetCode integration details, and [DESIGN.md](docs/DESIGN.md) for confirmed desktop UI decisions.

## Pull requests

Open pull requests against `main`; use a draft while work is in progress or to request early feedback.

- Keep each PR focused on one problem, without unrelated refactoring or formatting.
- Follow existing Go patterns, prefer the standard library and existing dependencies, and format changed Go files with `gofmt`.
- Test changed behavior and regressions, especially authentication boundaries, API handling, and draft preservation.
- Update relevant documentation only when the change affects it. Write documentation in English and describe confirmed behavior.
- Review your own diff, resolve merge conflicts, and exclude build artifacts, credentials, and local account data.
- Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for commit messages.

You are responsible for understanding and verifying everything you submit, including AI-assisted code and text.

In the PR description, explain the problem and resulting behavior, link any related issue, and summarize verification results and remaining limitations. Attach screenshots or a short recording for visible UI changes, with private data removed.

Respond to review feedback and keep follow-up changes within scope. Reviews depend on maintainer availability; a PR may be declined if it does not fit the project's direction or maintenance needs.

### Checks

Run the applicable checks before requesting review:

| Command | What it checks | When to run |
| --- | --- | --- |
| `go test ./...` | Automated tests across all Go packages | All code changes |
| `go vet ./...` | Common Go coding errors through static analysis | All code changes |
| `go tool mygo vet .` | MyGo UI usage and lifecycle issues | All code changes |
| `go test -race ./...` | Data races during test execution | Concurrency changes, on supported hosts |
| `go tool mygo build` | Application compilation and local packaging | Startup, dependency, resource, or packaging changes |

For UI, editor, or login changes, also run the app and check the affected workflow, including keyboard interaction and draft preservation. When modifying platform-specific files or build tags (such as `dock_icon_darwin.go` / `dock_icon_other.go`), also verify cross-platform compilation (`GOOS=linux` and `GOOS=windows`) where possible. Report the platforms and LeetCode sites tested, along with any checks you could not complete.

For documentation-only changes, check accuracy, links, and formatting; application tests are not required.

### Optional live API check

When changing public LeetCode requests, you can also test against the real services:

```sh
DECLIC_LIVE_API=1 go test ./internal/leetcode -run TestLiveGuest -v -count=1
```

`DECLIC_LIVE_API=1` enables the otherwise skipped live tests. The `-run TestLiveGuest` filter runs both `TestLiveGuest` and `TestLiveGuestArticles`, shows detailed output, and bypasses cached test results.

They check guest reads (catalog, daily challenge, search, detail, editorial, and community solutions) on both `leetcode.cn` and `leetcode.com`, without logging in or submitting code. They are optional for PRs and require network access; site restrictions can affect the result. Respect those restrictions.

This test does not cover account workflows, which require separate manual login. Never use real account credentials in CI.
