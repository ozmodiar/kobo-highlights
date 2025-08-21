package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "kobo-highlights",
		Usage: "Extract highlights from a KoboReader.sqlite database",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "kobo-db",
				Usage:    "Path to the KoboReader.sqlite file",
				Required: true,
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of highlights to fetch (omit or 0 = all)",
			},
			&cli.StringFlag{
				Name:    "notion-token",
				Usage:   "Notion integration token (or set NOTION_TOKEN env var)",
				EnvVars: []string{"NOTION_TOKEN"},
			},
			&cli.StringFlag{
				Name:    "notion-database",
				Usage:   "Notion database ID (or set NOTION_DB env var)",
				EnvVars: []string{"NOTION_DB"},
			},
			&cli.BoolFlag{
				Name:  "notion-sync",
				Usage: "Create/update a Notion page per book (Titel property)",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			dbPath := c.String("kobo-db")
			limit := c.Int("limit")
			var nc *NotionClient
			if c.Bool("notion-sync") {
				token := strings.TrimSpace(c.String("notion-token"))
				dbid := strings.TrimSpace(c.String("notion-database"))
				if token == "" || dbid == "" {
					return fmt.Errorf("notion-sync requested but notion-token or notion-database missing")
				}
				nc = NewNotionClient(token, dbid)
			}
			return readHighlights(dbPath, limit, nc)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func readHighlights(dbPath string, limit int, notion *NotionClient) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	baseQuery := `
		SELECT c.Title, COALESCE(c.Attribution, ''), b.Text, b.DateCreated
		FROM Bookmark b
		JOIN content c ON c.ContentID = b.VolumeID
		WHERE b.Text IS NOT NULL AND LENGTH(TRIM(b.Text)) > 0
		ORDER BY c.Title ASC, b.DateCreated DESC`

	var rows *sql.Rows
	if limit > 0 {
		q := baseQuery + " LIMIT ?"
		rows, err = db.Query(q, limit)
	} else {
		rows, err = db.Query(baseQuery)
	}
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	type highlight struct {
		text string
		date string
	}
	type bookGroup struct {
		author     string
		highlights []highlight
	}

	grouped := make(map[string]*bookGroup)
	order := make([]string, 0) // preserve title order encountered

	for rows.Next() {
		var title, author, text, date string
		if err := rows.Scan(&title, &author, &text, &date); err != nil {
			log.Printf("failed to scan row: %v", err)
			continue
		}
		if _, ok := grouped[title]; !ok {
			grouped[title] = &bookGroup{author: author, highlights: []highlight{}}
			order = append(order, title)
		}
		grouped[title].highlights = append(grouped[title].highlights, highlight{text: text, date: date})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration error: %w", err)
	}

	// To ensure deterministic alphabetical grouping even if query order changes later,
	// we sort the collected titles (already ordered by title ASC in query, but double safe).
	sort.Strings(order)

	for _, title := range order {
		g := grouped[title]
		fmt.Println("====================")
		if g.author != "" {
			fmt.Printf("%s (%s)\n", title, g.author)
		} else {
			fmt.Printf("%s\n", title)
		}
		for i, h := range g.highlights {
			truncated := truncateClean(h.text, 100)
			fmt.Printf("  %2d. %s\n", i+1, truncated)
		}
		fmt.Println()
	}
	if notion != nil {
		for _, title := range order {
			g := grouped[title]
			// Collect all highlights for this book as strings
			highlights := make([]string, len(g.highlights))
			for i, h := range g.highlights {
				highlights[i] = h.text
			}
			if err := notion.EnsureBookPage(title, g.author, highlights); err != nil {
				log.Printf("notion sync failed for '%s': %v", title, err)
			}
		}
	}
	return nil
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
