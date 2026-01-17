package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"

	"github.com/ozmodiar/kobo-highlights/formats"
)

// Console preview helper length.
const previewLen = 100

// cliResolver adapts *cli.Context to the FlagValueResolver interface.
type cliResolver struct{ ctx *cli.Context }

func (r cliResolver) String(name string) string { return r.ctx.String(name) }

func main() {
	// Build dynamic exporter flags
	exporterNames := formats.ListFormatNames()
	baseFlags := []cli.Flag{
		&cli.StringFlag{Name: "kobo-db", Usage: "Path to the KoboReader.sqlite file", Required: true},
		&cli.IntFlag{Name: "limit", Usage: "Maximum number of highlights to fetch (omit or 0 = all)"},
		&cli.BoolFlag{Name: "list-formats", Usage: "List available output formats and exit"},
		&cli.StringFlag{Name: "format", Usage: "Output format (one of: " + strings.Join(exporterNames, ", ") + ")"},
		&cli.BoolFlag{Name: "debug", Usage: "Enable verbose debug logging (same as setting KOBO_DEBUG=1)"},
	}
	// Append exporter-specific flags (all added; only used when chosen)
	for _, name := range exporterNames {
		if f, ok := formats.GetFormatFactory(name); ok {
			for _, fp := range f.Flags {
				if cf, ok2 := fp.CLIFlag().(cli.Flag); ok2 {
					baseFlags = append(baseFlags, cf)
				} else if cfp, ok3 := fp.CLIFlag().(*cli.StringFlag); ok3 { // handle pointer types
					baseFlags = append(baseFlags, cfp)
				}
			}
		}
	}
	app := &cli.App{
		Name:  "kobo-highlights",
		Usage: "Extract highlights from a KoboReader.sqlite database",
		Flags: baseFlags,
		Action: func(c *cli.Context) error {
			if c.Bool("list-formats") {
				fmt.Println("Available formats:")
				for _, n := range exporterNames {
					fmt.Printf("  - %s\n", n)
				}
				return nil
			}
			dbPath := c.String("kobo-db")
			limit := c.Int("limit")
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format == "" {
				return fmt.Errorf("--format required unless --list-formats is used")
			}
			factory, ok := formats.GetFormatFactory(format)
			if !ok {
				return fmt.Errorf("unknown format '%s' (available: %s)", format, strings.Join(exporterNames, ", "))
			}

			debug := c.Bool("debug")
			books, err := fetchBooks(dbPath, limit, debug)
			if err != nil {
				return err
			}
			printConsolePreview(books)

			// Resolver using cli.Context
			resolver := cliResolver{c}
			exporter, err := factory.Build(resolver)
			if err != nil {
				return err
			}
			if err := exporter.Export(books); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s export complete\n", exporter.Name())
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// fetchBooks queries the DB and returns a slice of Book structs.
func fetchBooks(dbPath string, limit int, debug bool) ([]formats.Book, error) {
	// Ensure the file exists before opening; opening a non-existent file without read-only mode would create an empty DB.
	if fi, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("database file not found: %s", dbPath)
		}
		return nil, fmt.Errorf("unable to stat database file: %w", err)
	} else if fi.Size() < 1024 { // heuristic: Kobo DBs are typically several MB; extremely small likely wrong file
		log.Printf("warning: database file is very small (%d bytes) – is this the correct KoboReader.sqlite?", fi.Size())
		if debug {
			log.Printf("DEBUG: db=%s size=%d bytes (suspiciously small)", dbPath, fi.Size())
		}
	} else if debug {
		log.Printf("DEBUG: db=%s size=%d bytes", dbPath, fi.Size())
	}

	// Open in read-only mode to avoid accidental creation.
	// Use a URI so we can set pragmas; no escaping needed for simple paths.
	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", filepath.Clean(dbPath))
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Verify Bookmark table exists before running main query.
	var tableName string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='Bookmark'`).Scan(&tableName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no such table") {
			// List available tables for diagnostics.
			rows, listErr := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
			available := []string{}
			if listErr == nil {
				defer rows.Close()
				for rows.Next() {
					var n string
					if scanErr := rows.Scan(&n); scanErr == nil {
						available = append(available, n)
					}
				}
			}
			if debug {
				log.Printf("DEBUG: Bookmark table missing; available tables: %s", strings.Join(available, ", "))
			}
			hint := "Ensure you passed the KoboReader.sqlite from the device (not BookReader.sqlite or another file)."
			if len(available) == 0 {
				hint += " No tables were found – the file might be empty or corrupted."
			} else {
				hint += " Available tables: " + strings.Join(available, ", ")
			}
			return nil, fmt.Errorf("required table 'Bookmark' not found. %s", hint)
		}
		return nil, fmt.Errorf("failed to inspect schema: %w", err)
	}
	if debug {
		log.Printf("DEBUG: Found Bookmark table")
	}

	baseQuery := `
		SELECT c.Title, COALESCE(c.Attribution, ''), b.Text, b.DateCreated
		FROM Bookmark b
		JOIN content c ON c.ContentID = b.VolumeID
		WHERE b.Text IS NOT NULL AND LENGTH(TRIM(b.Text)) > 0
		ORDER BY c.Title ASC,
		         b.ContentID ASC,
		         CAST(SUBSTR(b.StartContainerPath, INSTR(b.StartContainerPath, '.')+1,
		              INSTR(SUBSTR(b.StartContainerPath, INSTR(b.StartContainerPath, '.')+1), '.')-1) AS INTEGER) ASC,
		         b.StartOffset ASC`

	var rows *sql.Rows
	if limit > 0 {
		q := baseQuery + " LIMIT ?"
		rows, err = db.Query(q, limit)
	} else {
		rows, err = db.Query(baseQuery)
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string]*formats.Book)
	order := make([]string, 0)
	for rows.Next() {
		var title, author, text, date string
		if err := rows.Scan(&title, &author, &text, &date); err != nil {
			log.Printf("failed to scan row: %v", err)
			continue
		}
		if _, ok := grouped[title]; !ok {
			grouped[title] = &formats.Book{Title: title, Author: author, Highlights: []formats.Highlight{}}
			order = append(order, title)
		}
		grouped[title].Highlights = append(grouped[title].Highlights, formats.Highlight{Text: text, Date: date})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	sort.Strings(order)
	books := make([]formats.Book, 0, len(order))
	for _, t := range order {
		books = append(books, *grouped[t])
	}
	return books, nil
}

// printConsolePreview prints a deterministic summary to stdout.
func printConsolePreview(books []formats.Book) {
	for _, b := range books {
		fmt.Println("====================")
		if b.Author != "" {
			fmt.Printf("%s (%s)\n", b.Title, b.Author)
		} else {
			fmt.Printf("%s\n", b.Title)
		}
		for i, h := range b.Highlights {
			truncated := truncateClean(h.Text, previewLen)
			fmt.Printf("  %2d. %s\n", i+1, truncated)
		}
		fmt.Println()
	}
}

// truncateClean trims whitespace, replaces internal newlines with spaces, and truncates to max characters (rune-safe).
func truncateClean(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	// iterate runes to safe cut
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
