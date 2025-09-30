package formats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

// NotionClient is a minimal client for creating pages in a database.
type NotionClient struct {
	httpClient    *http.Client
	token         string
	databaseID    string
	titlePropName string
	resolvedTitle bool
}

func NewNotionClient(token, databaseID string) *NotionClient {
	return &NotionClient{httpClient: &http.Client{Timeout: 15 * time.Second}, token: token, databaseID: databaseID, titlePropName: "Title"}
}

// NotionFormat implements Format using an underlying NotionClient.
type NotionFormat struct{ Client *NotionClient }

func (n *NotionFormat) Name() string { return "notion" }

func (n *NotionFormat) Export(books []Book) error {
	if n.Client == nil {
		return fmt.Errorf("nil Notion client")
	}
	for _, b := range books {
		highlights := make([]string, len(b.Highlights))
		for i, h := range b.Highlights {
			highlights[i] = h.Text
		}
		if err := n.Client.EnsureBookPage(b.Title, b.Author, highlights); err != nil {
			return fmt.Errorf("notion export '%s': %w", b.Title, err)
		}
	}
	return nil
}

// EnsureBookPage creates a page for the book (Title + optional Author) and appends highlight blocks.
func (n *NotionClient) EnsureBookPage(title, author string, highlights []string) error {
	if n == nil {
		return nil
	}
	if !n.resolvedTitle {
		_ = n.resolveTitlePropertyName()
	}
	notionTitle := title
	if author != "" {
		notionTitle = fmt.Sprintf("%s (%s)", title, author)
	}
	exists, err := n.pageExistsByTitle(notionTitle)
	if err != nil {
		return fmt.Errorf("check existing page: %w", err)
	}
	if exists {
		return nil
	}
	props := map[string]any{n.titlePropName: map[string]any{"title": []map[string]any{{"text": map[string]string{"content": notionTitle}}}}}
	if author != "" {
		props["Author"] = map[string]any{"rich_text": []map[string]any{{"text": map[string]string{"content": author}}}}
	}
	payload := map[string]any{"parent": map[string]string{"database_id": n.databaseID}, "properties": props}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notion payload: %w", err)
	}
	createReq := func(p []byte) (*http.Response, error) {
		req, err := http.NewRequest("POST", "https://api.notion.com/v1/pages", bytes.NewReader(p))
		if err != nil {
			return nil, fmt.Errorf("build notion request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+n.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Notion-Version", "2022-06-28")
		return n.httpClient.Do(req)
	}
	resp, err := createReq(body)
	if err != nil {
		return fmt.Errorf("perform notion request: %w", err)
	}
	if resp.StatusCode == 400 && author != "" { // maybe Author property not defined
		resp.Body.Close()
		delete(props, "Author")
		payload["properties"] = props
		body2, _ := json.Marshal(payload)
		resp, err = createReq(body2)
		if err != nil {
			return fmt.Errorf("retry notion request (without Author): %w", err)
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notion create page error: %s – %s", resp.Status, truncateForLog(string(b), 300))
	}
	var pageResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
		return fmt.Errorf("decode page create response: %w", err)
	}
	pageID := pageResp.ID
	if pageID == "" {
		return fmt.Errorf("no page ID returned from Notion")
	}
	blocks := make([]map[string]any, 0, len(highlights)*2)
	for i, h := range highlights {
		blocks = append(blocks, map[string]any{
			"object": "block",
			"type":   "quote",
			"quote":  map[string]any{"rich_text": []map[string]any{{"type": "text", "text": map[string]string{"content": h}}}},
		})
		if i < len(highlights)-1 {
			blocks = append(blocks, map[string]any{"object": "block", "type": "paragraph", "paragraph": map[string]any{"rich_text": []map[string]any{}}})
		}
	}
	for i := 0; i < len(blocks); i += 100 {
		end := i + 100
		if end > len(blocks) {
			end = len(blocks)
		}
		batch := blocks[i:end]
		appendPayload := map[string]any{"children": batch}
		appendBody, err := json.Marshal(appendPayload)
		if err != nil {
			return fmt.Errorf("marshal append payload: %w", err)
		}
		url := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children", pageID)
		req, err := http.NewRequest("PATCH", url, bytes.NewReader(appendBody))
		if err != nil {
			return fmt.Errorf("build append request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+n.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Notion-Version", "2022-06-28")
		resp, err := n.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("perform append request: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("notion append error: %s – %s", resp.Status, truncateForLog(string(b), 300))
		}
	}
	return nil
}

func (n *NotionClient) pageExistsByTitle(title string) (bool, error) {
	if !n.resolvedTitle {
		_ = n.resolveTitlePropertyName()
	}
	queryPayload := map[string]any{"page_size": 1, "filter": map[string]any{"property": n.titlePropName, "title": map[string]any{"equals": title}}}
	body, err := json.Marshal(queryPayload)
	if err != nil {
		return false, fmt.Errorf("marshal query payload: %w", err)
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", n.databaseID), bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("build query request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("perform query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("query API error: %s – %s", resp.Status, truncateForLog(string(b), 200))
	}
	var qr struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return false, fmt.Errorf("decode query response: %w", err)
	}
	return len(qr.Results) > 0, nil
}

func (n *NotionClient) resolveTitlePropertyName() error {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s", n.databaseID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Notion-Version", "2022-06-28")
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("fetch database failed: %s", resp.Status)
	}
	var db struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&db); err != nil {
		return err
	}
	for name, meta := range db.Properties {
		if meta.Type == "title" {
			if name != "Title" {
				n.titlePropName = name
			}
			break
		}
	}
	n.resolvedTitle = true
	return nil
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// registration
type notionTokenFlag struct{}

func (notionTokenFlag) CLIFlag() any {
	return &cli.StringFlag{Name: "notion-token", Usage: "Notion integration token (or NOTION_TOKEN)", EnvVars: []string{"NOTION_TOKEN"}}
}

type notionDBFlag struct{}

func (notionDBFlag) CLIFlag() any {
	return &cli.StringFlag{Name: "notion-database", Usage: "Notion database ID (or NOTION_DB)", EnvVars: []string{"NOTION_DB"}}
}

func init() {
	RegisterFormat(&FormatFactory{
		Name:  "notion",
		Flags: []FlagProvider{notionTokenFlag{}, notionDBFlag{}},
		Build: func(r FlagValueResolver) (Format, error) {
			token := strings.TrimSpace(r.String("notion-token"))
			dbid := strings.TrimSpace(r.String("notion-database"))
			if token == "" || dbid == "" {
				return nil, fmt.Errorf("--notion-token and --notion-database required for format notion")
			}
			return &NotionFormat{Client: NewNotionClient(token, dbid)}, nil
		},
	})
}
