package formats

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

// MarkdownFormat writes one markdown file per book.
type MarkdownFormat struct{ Dir string }

func (m *MarkdownFormat) Name() string { return "markdown" }

func (m *MarkdownFormat) Export(books []Book) error {
	if m.Dir == "" {
		return fmt.Errorf("markdown format: empty directory")
	}
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	for _, b := range books {
		filename := sanitizeFilename(b.Title)
		if b.Author != "" {
			filename = sanitizeFilename(b.Title + "-" + b.Author)
		}
		path := filepath.Join(m.Dir, filename+".md")
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create file %s: %w", path, err)
		}
		if b.Author != "" {
			fmt.Fprintf(f, "# %s (%s)\n\n", b.Title, b.Author)
		} else {
			fmt.Fprintf(f, "# %s\n\n", b.Title)
		}
		for _, h := range b.Highlights {
			text := strings.TrimSpace(h.Text)
			if text == "" {
				continue
			}
			fmt.Fprintf(f, "> %s\n\n", strings.ReplaceAll(text, "\n", " "))
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close file %s: %w", path, err)
		}
	}
	return nil
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "-",
	)
	s = replacer.Replace(s)
	s = strings.Join(strings.Fields(s), "-")
	if s == "" {
		return "book"
	}
	return s
}

// registration
type markdownDirFlag struct{}

func (markdownDirFlag) CLIFlag() any {
	return &cli.StringFlag{Name: "markdown-dir", Usage: "Directory for markdown output (required when --format markdown)"}
}

func init() {
	RegisterFormat(&FormatFactory{
		Name:  "markdown",
		Flags: []FlagProvider{markdownDirFlag{}},
		Build: func(r FlagValueResolver) (Format, error) {
			dir := strings.TrimSpace(r.String("markdown-dir"))
			if dir == "" {
				return nil, fmt.Errorf("--markdown-dir required for format markdown")
			}
			return &MarkdownFormat{Dir: dir}, nil
		},
	})
}
