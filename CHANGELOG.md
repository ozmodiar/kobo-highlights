# Changelog

All notable changes to this project will be documented in this file.

The format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and uses semantic versioning.

## [2.0.1] - 2026-01-17
### Fixed
- Highlights are now exported in chronological order (as they appear in the book) instead of reverse order.

### Changed
- README download instructions now include architecture-specific binary names for different platforms (darwin_arm64, darwin_amd64, linux_arm64, linux_amd64).

## [2.0.0] - 2025-09-30
### Added
- Required `--format` flag selecting `notion` or `markdown` output.
- Markdown format (`--format markdown --markdown-dir <dir>`): per-book file with quote blocks.
- `--list-formats` flag to enumerate available formats.
- Modular format registry (`formats/` directory) allowing new formats without modifying `main.go`.
- Breaking Changes section in README.

### Removed
- Deprecated `--notion-sync` flag (use `--format notion`).
- Legacy root exporter and notion files replaced by `formats/` package.

### Changed
- CLI validation now enforces presence of scenario-specific flags.

## [1.0.0] - 2025-08-26
- Initial functionality: console grouping, optional Notion sync (`--notion-sync`).
- Added early markdown export (experimental) prior to v2 flag rework.

[2.0.1]: https://github.com/ozmodiar/kobo-highlights/releases/tag/v2.0.1
[2.0.0]: https://github.com/ozmodiar/kobo-highlights/releases/tag/v2.0.0
[1.0.0]: https://github.com/ozmodiar/kobo-highlights/releases/tag/v1.0.0