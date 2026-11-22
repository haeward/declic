# Changelog

All notable changes to Declic will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-11-22

First public preview release of Declic for macOS 14.0+ (Apple Silicon & Intel).

### Added

- **Problem Library & Daily Challenge**: Full per-site public catalog indexing (`leetcode.com` and `leetcode.cn`) with 24-hour local disk caching, instant search, multi-facet filtering (difficulty, topics, access, solved status), sorting, pagination, Pick One, keyboard list navigation (`↑` / `↓` / `Enter`), Quick Open (`⌘P` / `⌘K`), and a site-aware Daily Challenge card.
- **Dual-Site Isolation & Official Sign-In**: Completely isolated `leetcode.com` and `leetcode.cn` WebView sessions, accounts, solved flags, caches, and drafts. Sign-in runs on the official website without password capture, with automatic session verification, 5-minute in-memory verification caching, profile/avatar enrichment, and optional session restore on launch.
- **Native Workspace & Rich Content**:
  - **Description**: Isolated ephemeral WebView rendering sanitized HTML with native Presentation MathML typesetting, CN bilingual toggle (`中文 / EN`), collapsible table of contents, separate Hints, and clickable Similar Questions.
  - **Editorial & Solutions**: Native Markdown reader with inline and display TeX math, GPU-rendered Mermaid diagrams, interactive walkthrough slides, embedded LeetCode Playground multi-language code tabs, and isolated inline video solution players.
  - **Submissions**: Signed-in submission history with status/language filters, syntax-highlighted code inspection, and one-click `Use this code`.
- **Native Code Editor & Local Drafts**: Support for 25 LeetCode languages with syntax highlighting, zero-dependency code formatting, language-conventional indentation (`Auto`, `2`, or `4` spaces), template reset, Quick Editor Settings, Focus Editor mode (`⌘\`), and automatic versioned local draft persistence separated by site, account, problem, and language.
- **Online Judge**: Editable testcases, `Run` (`⌘↩`) with character-level Output vs. Expected diff, and `Submit` (`⌘⇧↩`) with verdict, runtime/memory metrics, percentile beats, and immediate solved-status refresh.
- **Desktop Experience & Settings**: System/Light/Dark appearances synced with native window chrome, contrast & density controls, description/editor typography settings, macOS alternate Dock icons, startup continuity (`Open on launch` and `Restore session on launch`), and Storage controls (`Show in Finder` / `Clear caches`).
- **Packaging & Automation**: macOS dual-architecture packaging (`darwin/arm64` and `darwin/amd64` `.dmg` with `SHA256SUMS.txt`), GitHub Actions CI, `govulncheck` security workflow, Dependabot, and automated draft release workflow.

### Known Limitations

- Verified primarily on macOS 14.0+; Windows and Linux prebuilt installers are planned after native runtime validation.
- macOS preview `.dmg` packages are ad-hoc signed and not notarized; clear `com.apple.quarantine` once after moving `Declic.app` to `/Applications`.

[Unreleased]: https://github.com/haeward/declic/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/haeward/declic/releases/tag/v0.1.0
