# kobo-highlights

Extract reading highlights from a Kobo eReader database. Output to different formats (Notion pages or per‑book Markdown files).

## Features
- Query all non-empty highlights (optionally limit the number returned)
- Group output by book (Title (Author))
- Notion format (page per book, quote blocks, batched)
- Markdown format (one file per book)

## Prerequisites
- Access to your Kobo device's `KoboReader.sqlite` (usually on the mounted device at `.kobo/KoboReader.sqlite`)
- (Only if building from source) Go 1.24+ (earlier 1.21+ likely fine; module sets 1.24.3)

## Quick Start (Binary)
Download the latest release for your platform from the GitHub Releases page and run it.

```bash
# macOS ARM64 (Apple Silicon)
curl -L -o kobo-highlights https://github.com/ozmodiar/kobo-highlights/releases/latest/download/kobo-highlights_darwin_arm64
chmod +x kobo-highlights

# macOS AMD64 (Intel)
curl -L -o kobo-highlights https://github.com/ozmodiar/kobo-highlights/releases/latest/download/kobo-highlights_darwin_amd64
chmod +x kobo-highlights

# Linux ARM64
curl -L -o kobo-highlights https://github.com/ozmodiar/kobo-highlights/releases/latest/download/kobo-highlights_linux_arm64
chmod +x kobo-highlights

# Linux AMD64
curl -L -o kobo-highlights https://github.com/ozmodiar/kobo-highlights/releases/latest/download/kobo-highlights_linux_amd64
chmod +x kobo-highlights

# Optionally, copy the KoboReader database locally
cp /Volumes/KOBOeReader/.kobo/KoboReader.sqlite ./KoboReader.sqlite

./kobo-highlights --kobo-db ./KoboReader.sqlite --list-formats
```

## Formats
List available formats:
```bash
./kobo-highlights --kobo-db ./KoboReader.sqlite --list-formats
```

Use one with `--format`:
- `--format notion` – create (if absent) a Notion page per book
- `--format markdown` – write per-book markdown files

`--format` is required unless `--list-formats` is used.

## Common Flags
| Flag | Required? | Description |
|------|-----------|-------------|
| `--kobo-db` | Yes | Path to `KoboReader.sqlite` |
| `--limit` | No | Max highlights (after grouping). 0 = all |
| `--list-formats` | No | Print available formats and exit |
| `--format` | Yes* | One of the registered formats (currently `notion` or `markdown`). *Not required with `--list-formats` |
| `--notion-token` | Yes (format=notion) | Notion integration token (or env `NOTION_TOKEN`) |
| `--notion-database` | Yes (format=notion) | Notion database ID (or env `NOTION_DB`) |
| `--markdown-dir` | Yes (format=markdown) | Output directory for markdown files |
| `--debug` | No | Verbose diagnostics (prints DB size, table info) |

## Examples
```bash
# Markdown format (all highlights)
./kobo-highlights --kobo-db ./KoboReader.sqlite --format markdown --markdown-dir ./md

# Markdown format (limit 25)
./kobo-highlights --kobo-db ./KoboReader.sqlite --format markdown --markdown-dir ./md --limit 25

# Notion format
export NOTION_TOKEN=ntn_xxx
export NOTION_DB=your_database_id
./kobo-highlights --kobo-db ./KoboReader.sqlite --format notion
```

## Notion Format Details
Behavior:
- Skips creation if a page with the same computed title already exists
- Page title format: `Book Title (Author)` (author omitted if empty)
- Highlights appended as quote blocks separated by blank paragraphs
- Blocks uploaded in batches ≤100 (Notion API limit)

## Markdown Format Details
Each file contains:
- H1 heading: `Book Title (Author)`
- Each highlight rendered as a block quote (`> text`)
- Blank line between quotes

File name pattern: sanitized `Title[-Author].md` (unsafe characters removed, spaces collapsed to dashes).

## Console Sample
```
====================
Book Title (Author)
   1. First highlight...
   2. Second highlight...
```

## Sample Markdown File
```
# Book Title (Author)

> First highlight...

> Second highlight...
```

## Breaking Changes (v2)
- Removed deprecated `--notion-sync` flag. Use `--format notion` instead.
- Introduced dynamic formats system. Use `--list-formats` to inspect available options.

## Exit Codes
- 0 success
- Non-zero fatal error (e.g. cannot open DB, query failure, validation error, Notion request error)

## Troubleshooting
| Issue | Fix |
|-------|-----|
| `no such file or directory` | Verify the `--kobo-db` path |
| Empty output | Ensure the source DB actually contains highlights |
| `--format` error | Must be exactly `markdown` or `notion` |
| Notion API error | Check token/database, ensure integration has access |
| SQLite driver issues | Ensure system SQLite present (`libsqlite3`). On Linux install `libsqlite3-dev` |
| `required table 'Bookmark' not found` | Confirm you used `KoboReader.sqlite` (not `BookReader.sqlite`), recopy from device, optionally run with `--debug` to list tables |

## Building from Source (Optional)
Only needed if you want the very latest `main` changes or to modify the tool.

```bash
git clone https://github.com/ozmodiar/kobo-highlights.git
cd kobo-highlights
go build -o kobo-highlights .
./kobo-highlights --kobo-db ./KoboReader.sqlite --list-formats
```

Dev helpers:
```bash
go mod tidy
go vet ./...
```

## Future Enhancements
- Additional formats (e.g. JSON)
- Per-book filtering
- Output formatting templates

## License
MIT License – see `LICENSE`.

## Contributing
PRs and issues welcome.
