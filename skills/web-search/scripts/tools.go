package websearchscripts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	// "os"
	"regexp"
	"strings"
	"time"

	"pipeline/internal/tool"
)

func Tools() []tool.Tool {
	return []tool.Tool{TavilySearch{}, TavilyExtract{}, FetchPage{}}
}

const tavilyBase = "https://api.tavily.com"

func tavilyPost(endpoint string, payload map[string]any) ([]byte, error) {
	key := "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"
	if key == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY not set — get a free key at app.tavily.com")
	}
	payload["api_key"] = key

	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest(http.MethodPost, tavilyBase+endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case 200:
		return raw, nil
	case 401:
		return nil, fmt.Errorf("invalid TAVILY_API_KEY")
	case 429:
		return nil, fmt.Errorf("Tavily rate limit exceeded")
	default:
		return nil, fmt.Errorf("Tavily API error %d: %s", resp.StatusCode, raw)
	}
}

// ── TavilySearch ──────────────────────────────────────────────────────────────

type TavilySearch struct{}

func (t TavilySearch) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name: "tavily_search",
		Description: "Search the web. Returns AI answer + full extracted content per result. " +
			"Stop calling once you have enough to answer. " +
			"topic: news=current events, finance=financial data, general=everything else.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":          map[string]any{"type": "string"},
				"topic":          map[string]any{"type": "string", "enum": []string{"general", "news", "finance"}},
				"time_range":     map[string]any{"type": "string", "enum": []string{"day", "week", "month", "year"}},
				"max_results":    map[string]any{"type": "integer"},
				"include_domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"exclude_domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
			"required": []string{"query"},
		},
	}}
}

func (t TavilySearch) Run(args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("missing 'query'")
	}

	payload := map[string]any{
		"query":               query,
		"search_depth":        "advanced",
		"include_answer":      true,
		"include_raw_content": true,
		"chunks_per_source":   3,
		"max_results":         5,
	}
	if v, _ := args["topic"].(string); v != "" {
		payload["topic"] = v
	}
	if v, _ := args["time_range"].(string); v != "" {
		payload["time_range"] = v
	}
	if v, ok := args["max_results"].(float64); ok && v > 0 {
		n := int(v)
		if n > 10 {
			n = 10
		}
		payload["max_results"] = n
	}
	if v, ok := args["include_domains"].([]any); ok {
		payload["include_domains"] = v
	}
	if v, ok := args["exclude_domains"].([]any); ok {
		payload["exclude_domains"] = v
	}

	raw, err := tavilyPost("/search", payload)
	if err != nil {
		return "", err
	}

	var result struct {
		Query   string `json:"query"`
		Answer  string `json:"answer"`
		Results []struct {
			Title      string  `json:"title"`
			URL        string  `json:"url"`
			Content    string  `json:"content"`
			RawContent string  `json:"raw_content"`
			Score      float64 `json:"score"`
		} `json:"results"`
	}
	json.Unmarshal(raw, &result)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search: %q\n\n", result.Query))
	if result.Answer != "" {
		sb.WriteString(fmt.Sprintf("Direct answer: %s\n\n", result.Answer))
	}
	for i, r := range result.Results {
		content := r.RawContent
		if content == "" {
			content = r.Content
		}
		if len(content) > 1500 {
			content = content[:1500] + "..."
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n    URL: %s\n    Score: %.2f\n    %s\n\n",
			i+1, r.Title, r.URL, r.Score,
			strings.ReplaceAll(content, "\n", "\n    ")))
	}
	return sb.String(), nil
}

// ── TavilyExtract ─────────────────────────────────────────────────────────────

type TavilyExtract struct{}

func (t TavilyExtract) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name:        "tavily_extract",
		Description: "Extract full content from specific URLs. Better than fetch_page. Use when search result content is thin.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"urls":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"query": map[string]any{"type": "string", "description": "Optional — reranks for relevance"},
			},
			"required": []string{"urls"},
		},
	}}
}

func (t TavilyExtract) Run(args map[string]any) (string, error) {
	rawURLs, _ := args["urls"].([]any)
	if len(rawURLs) == 0 {
		return "", fmt.Errorf("missing 'urls'")
	}

	urls := make([]string, 0, len(rawURLs))
	for _, u := range rawURLs {
		if s, ok := u.(string); ok && s != "" {
			urls = append(urls, s)
		}
	}
	if len(urls) > 5 {
		urls = urls[:5]
	}

	payload := map[string]any{"urls": urls, "format": "markdown"}
	if q, _ := args["query"].(string); q != "" {
		payload["query"] = q
	}

	raw, err := tavilyPost("/extract", payload)
	if err != nil {
		return "", err
	}

	var result struct {
		Results []struct {
			URL        string `json:"url"`
			RawContent string `json:"raw_content"`
		} `json:"results"`
		FailedResults []struct {
			URL   string `json:"url"`
			Error string `json:"error"`
		} `json:"failed_results"`
	}
	json.Unmarshal(raw, &result)

	var sb strings.Builder
	for _, r := range result.Results {
		content := r.RawContent
		if len(content) > 4000 {
			content = content[:4000] + "\n[truncated]"
		}
		sb.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", r.URL, content))
	}
	for _, f := range result.FailedResults {
		sb.WriteString(fmt.Sprintf("Failed: %s — %s\n", f.URL, f.Error))
	}
	return sb.String(), nil
}

// ── FetchPage — HTTP fallback ─────────────────────────────────────────────────

type FetchPage struct{}

func (f FetchPage) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name:        "fetch_page",
		Description: "Fetch full text from a URL via HTTP. Last resort — use tavily_extract first.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"url": map[string]any{"type": "string"}},
			"required":   []string{"url"},
		},
	}}
}

func (f FetchPage) Run(args map[string]any) (string, error) {
	pageURL, _ := args["url"].(string)
	if !strings.HasPrefix(pageURL, "http") {
		return "", fmt.Errorf("invalid URL")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, pageURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Pipeline/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to fetch %s: %v", pageURL, err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 401 {
		return fmt.Sprintf("Access denied (%d) for %s", resp.StatusCode, pageURL), nil
	}
	if resp.StatusCode == 404 {
		return fmt.Sprintf("Not found (404): %s", pageURL), nil
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/pdf") {
		return fmt.Sprintf("PDF at %s — use tavily_extract instead", pageURL), nil
	}

	limited := io.LimitReader(resp.Body, 512*1024)
	raw, _ := io.ReadAll(limited)
	text := htmlToText(string(raw))

	if len(text) > 12000 {
		text = text[:12000] + "\n[truncated]"
	}
	return fmt.Sprintf("=== %s ===\n\n%s", pageURL, strings.TrimSpace(text)), nil
}

func htmlToText(html string) string {
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`),
		regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`),
		regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`),
		regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`),
	} {
		html = re.ReplaceAllString(html, "")
	}
	html = regexp.MustCompile(`(?i)</?(?:p|div|h[1-6]|li|br|section|article)[^>]*>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(html, " ")
	for old, new := range map[string]string{
		"&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": `"`, "&nbsp;": " ",
	} {
		html = strings.ReplaceAll(html, old, new)
	}
	html = regexp.MustCompile(`[ \t]+`).ReplaceAllString(html, " ")
	html = regexp.MustCompile(`\n{3,}`).ReplaceAllString(html, "\n\n")
	return strings.TrimSpace(html)
}