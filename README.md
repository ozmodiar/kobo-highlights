# kobo-highlights

Extract reading highlights from a Kobo eReader database. Optionally write them to a Notion database.

## Features
- Single, fast Go CLI binary
- Query all non-empty highlights (optionally limit the number returned)
- Group output by book (Title (Author))
- Optional sync of books + highlights to Notion (handles 100-block API limit with batching)

## Prerequisites
- Go 1.24+ (earlier 1.21+ likely fine; module sets 1.24.3)
- Access to your Kobo device's `KoboReader.sqlite` (usually on the mounted device at `.kobo/KoboReader.sqlite`)

You can copy the DB locally for faster iterations:
```bash
cp /Volumes/KOBOeReader/.kobo/KoboReader.sqlite ./KoboReader.sqlite
```

## Build
```bash
go build -o kobo-highlights .
```

## Run
```bash
# All highlights
./kobo-highlights --kobo-db ./KoboReader.sqlite

# Limit to first 10 highlights (after grouping order)
./kobo-highlights --kobo-db ./KoboReader.sqlite --limit 10
```
`--kobo-db` is required. If `--limit` is omitted or 0, all highlights are returned.

## Notion Integration
Sync each book (Title, Author) plus all its highlights as quote blocks in a Notion database.

```bash
export NOTION_TOKEN=ntn_xxx
export NOTION_DB=your_database_id
./kobo-highlights --kobo-db ./KoboReader.sqlite --notion-sync
```

Flags:
- `--notion-sync` enable syncing
- `--notion-token` override (else uses NOTION_TOKEN)
- `--notion-database` override (else uses NOTION_DB)

Behavior:
- Skips creating a page if one with the same Title already exists
- Page title format: `Book Title (Author)`
- Highlights appended as quote blocks separated by blank paragraphs
- Blocks are appended in batches ≤100 to satisfy Notion API limits

## Sample Output (console)
```
====================
Book Title (Author)
   1. First highlight...
   2. Second highlight...
```

## Sample Output (Notion)
Each page lists all highlights as quote blocks, ready for further notes.

## Exit Codes
- 0 success
- Non-zero fatal error (e.g. cannot open DB, query failure, Notion request error)

## Troubleshooting
| Issue | Fix |
|-------|-----|
| `no such file or directory` | Verify the `--kobo-db` path |
| Empty output | Ensure the source DB actually contains highlights |
| Notion API error: 400 | Automatic batching should prevent this; open an issue with the log output |
| SQLite driver issues | Ensure CGO can find system SQLite (`libsqlite3`). On Linux install `libsqlite3-dev` |

## Development
```bash
go mod tidy
go vet ./...
```

## Future Enhancements
- Export to Markdown / JSON
- Per-book filtering
- Output formatting templates

## License
MIT License – see the `LICENSE` file for details.

## Contributing
Feel free to open issues or PRs for enhancements.
