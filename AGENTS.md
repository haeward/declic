# AGENTS.md

Declic is a third-party, open-source LeetCode desktop client for `leetcode.cn` and `leetcode.com`.

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for component boundaries, [API.md](docs/API.md) for LeetCode integration, and [DESIGN.md](docs/DESIGN.md) for confirmed UI design decisions.

## Stack and Commands

- Go and MyGo Native UI (`ui`, `ui/editor`, and `ui/markdown`) provide the application, code editor, and native Editorial/Solutions reader.
- MyGo WebViews serve three isolated roles: per-site named sessions for official login, an ephemeral session for sanitized HTML problem statements, and a separate shared ephemeral session for inline video players.
- The Go standard library handles HTTP/GraphQL requests (`internal/leetcode`), local draft files (`internal/workspace`), and code formatting (`internal/editorutil`). `internal/render` uses `golang.org/x/net/html` and `mygo/ui` primitives for HTML sanitization, Markdown preprocessing, native TeX math, and Mermaid diagrams. `golang.org/x/image/webp` decodes WebP remote bitmaps.

Use the versions pinned in [go.mod](go.mod) and run commands from the repository root. `go tool mygo` invokes the pinned CLI.

| Command | Purpose |
| --- | --- |
| `go tool mygo doctor` | Check development prerequisites |
| `go tool mygo dev` | Build and run the development app |
| `go test ./...` | Run offline Go tests |
| `go vet ./...` | Run Go static analysis |
| `go tool mygo vet .` | Check MyGo UI usage and lifecycle |
| `go test -race ./...` | Check for data races on supported hosts |
| `go tool mygo build` | Compile and package the application locally |

## Working Conventions

- Check the branch and working tree before editing. Preserve user changes and keep work within the requested scope. Treat reviews, discussions, and plans as read-only unless edits are requested.
- Follow Go conventions and existing project patterns. Format changed Go files with `gofmt`, reuse existing code, and prefer the standard library. Add dependencies or abstractions only to meet a concrete need. Keep unrelated refactoring and formatting out of the change.
- Keep component boundaries intact: `internal/leetcode`, `internal/workspace`, and `internal/editorutil` remain pure standard-library packages without UI dependencies; `internal/render` stays decoupled from desktop application state and storage. Surface failures where the caller or user can act on them. Update the affected documentation when confirmed behavior or design decisions change.
- `main` is protected (pull requests and required status checks are enforced). Create a topic branch from `main` for changes, push that branch, and open a pull request instead of committing or pushing directly to `main`. Keep Conventional Commit subjects and PR titles concise (target ≤ 50 characters, max 72, putting details in the body or PR description). Obtain authorization before destructive operations or pushes.

## Desktop UI

- Verify APIs against the pinned MyGo version. Keep application and widget state across renders. Use UI elements only during the build pass that created them.
- Run blocking I/O off the window thread and apply results through `Window.Update`. Ignore results that arrive after the window closes or no longer apply to the active site, account, problem, or generation.
- Allow editing during snapshot saves. Use the editor's read-only state during operations that may replace the active document.
- Follow confirmed decisions for layout, components, and colors. Record approved design decisions in [DESIGN.md](docs/DESIGN.md) and keep the implementation consistent with them.

## Safety Rules

- Follow [SECURITY.md](SECURITY.md). Keep `leetcode.cn` and `leetcode.com` accounts, sessions, caches, and drafts separate. Verify the selected site's session identity before selecting account-specific data or invoking the online judge; invalidate cached sessions on sign-out, site switch, expiry, or auth errors. Keep guest requests free of credentials and respect authentication, membership restrictions, and rate limits.
- Keep secrets out of source control, logs, and test data. Treat remote content as untrusted: isolate description previews and inline video players from login sessions and privileged APIs, restrict video players to allowlisted HTTPS hosts or direct HTTPS video streams, and keep remote image/slide fetches HTTPS-only, cookie-free, and size-bounded.
- Never persist Premium content, credentials, or private submissions to public disk caches. Never automatically retry Run or Submit requests.
- Save pending edits before changing the site, account, problem, or language. When loading another document, apply its draft key and editor contents together after the load succeeds. Preserve empty drafts and keep the active text available on failure. An older save must leave newer edits marked as unsaved.
- Account for existing user data when changing application identity, storage paths, or formats. Never silently delete, overwrite, or migrate user data.

## Verification

- Run Go tests, Go static analysis, and MyGo vet for code changes. Add race checks for concurrency changes, cross-platform compilation checks (`GOOS=linux` / `GOOS=windows`) when touching platform-gated code or build tags, and a build for startup, dependency, resource, or packaging changes. Follow the [contribution checks](CONTRIBUTING.md#checks) for details.
- Add focused tests for behavior or regressions that existing tests miss. Prefer tests of observable application behavior and avoid duplicating framework coverage. Use synthetic data and temporary storage. Follow the [optional live API check](CONTRIBUTING.md#optional-live-api-check) when verifying real services, and keep it opt-in.
- Use `ui.NewTester` for interaction regressions where useful, and run affected UI workflows in the native app. For editor changes, check relevant keyboard input, focus, input composition, and draft preservation. Verify platform-specific behavior on the affected platform when available.
- For documentation changes, check links and examples. Render changed diagrams and verify that they represent the implementation.
- Report the checks run and any material gaps. Distinguish automated tests, cross-compilation, and native runtime verification.
