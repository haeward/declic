# Security

This document defines security and data-protection requirements for Declic. These requirements are development constraints, not a claim that the application has completed a security audit.

## Accounts and credentials

- Keep CN and COM accounts, browser sessions, credentials, and account-specific data isolated. Send credentials only to the selected site's HTTPS origin; do not forward them through cross-origin redirects.
- Keep guest requests free of account credentials. Validate session identity before accessing an account's local data.
- Let the official login page handle sign-in. Never collect passwords in native application controls or bypass authentication, challenges, rate limits, or membership restrictions.
- Keep cookies, tokens, passwords, and session storage out of source control, logs, screenshots, fixtures, and issue reports. Never use real account credentials in CI. Verified session credentials cached in memory (5-minute TTL) must never be written to disk and must be invalidated on sign-out, site switch, session expiry, or authentication errors.
- Signing out must clear the selected site's application session and account state without signing out the other site or deleting saved drafts.

## External content and execution

Treat descriptions, articles, HTML, URLs, API responses, and downloaded media as untrusted. Sanitize rendered content, restrict navigation and resource loading, and prevent untrusted pages from accessing privileged application APIs. Description previews and inline video players must run in ephemeral, untrusted sessions isolated from login cookies; inline video players are restricted to allowlisted HTTPS video providers or direct HTTPS video streams, and remote image/slide fetches must remain HTTPS-only, cookie-free, and size-bounded.

Do not execute user code, downloaded programs, or commands derived from external content without an explicitly defined execution boundary. Declic has no local code runner; Run and Submit execute remotely on the selected LeetCode site using a verified viewer session and never retry submissions automatically.

## Local data

Keep user-authored drafts separate from replaceable remote data. Public metadata caches (per-site catalog, non-paid problem details, and public editorials) must never persist Premium content, credentials, or private submissions. Validate path components before file access. Preserve existing drafts, including empty files, and surface read and write failures.

Never silently overwrite, delete, or migrate user data. Failed saves must preserve recoverable editor contents; changing sites or accounts must not redirect an outstanding save into another namespace. See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for the draft identity and persistence flow.

Drafts are stored as ordinary files, without application-level encryption. Account directory hashing does not protect their contents from other software with access to the same files.

## Reporting concerns

Do not publish credentials, private code, personal account data, or exploit details in a public issue. For a suspected vulnerability, use the repository's private vulnerability reporting option if it is available. Otherwise, request a private contact channel from the maintainer without disclosing sensitive details publicly.

Prepare the affected revision, operating system, impact, and minimal reproduction using synthetic data. Share sensitive reproduction details only through the agreed private channel. Never include live credentials.
