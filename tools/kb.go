package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// KBDocument represents a stored knowledge base entry
type KBDocument struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags"`
	CreatedAt string            `json:"created_at"`
	Words     map[string]int    `json:"words"` // word -> frequency
	TotalWords int              `json:"total_words"`
}

type kbStore struct {
	mu    sync.Mutex
	docs  map[string]*KBDocument
}

var kb = &kbStore{docs: make(map[string]*KBDocument)}

func kbPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".apexclaw", "kb.json")
}

func loadKB() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	data, err := os.ReadFile(kbPath())
	if err != nil {
		return
	}

	var docs []*KBDocument
	if err := json.Unmarshal(data, &docs); err != nil {
		return
	}

	for _, doc := range docs {
		kb.docs[doc.ID] = doc
	}
}

func persistKB() {
	kb.mu.Lock()
	docs := make([]*KBDocument, 0, len(kb.docs))
	for _, doc := range kb.docs {
		docs = append(docs, doc)
	}
	kb.mu.Unlock()

	path := kbPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.MarshalIndent(docs, "", "  ")
	os.WriteFile(path, data, 0644)
}

func init() {
	loadKB()
}

// stopWords are words that don't contribute to search
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "by": true, "is": true, "are": true,
	"was": true, "be": true, "been": true, "being": true, "have": true,
	"has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "could": true, "should": true, "may": true,
	"might": true, "can": true, "it": true, "its": true, "this": true,
	"that": true, "these": true, "those": true, "i": true, "you": true,
	"he": true, "she": true, "we": true, "they": true, "what": true,
	"which": true, "who": true, "when": true, "where": true, "why": true,
	"how": true, "all": true, "each": true, "every": true, "both": true,
	"few": true, "more": true, "most": true, "other": true, "some": true,
	"such": true, "no": true, "nor": true, "not": true, "only": true,
	"same": true, "so": true, "than": true, "too": true, "very": true,
}

func tokenizeKB(text string) []string {
	// Convert to lowercase, split on non-alphanumeric
	re := regexp.MustCompile(`[a-z0-9]+`)
	words := re.FindAllString(strings.ToLower(text), -1)
	var result []string
	for _, w := range words {
		if len(w) > 2 && !stopWords[w] {
			result = append(result, w)
		}
	}
	return result
}

func buildWordFreq(content string) map[string]int {
	words := tokenizeKB(content)
	freq := make(map[string]int)
	for _, w := range words {
		freq[w]++
	}
	return freq
}

// tfidfScore computes TF-IDF score of a document for given query
func tfidfScore(queryWords []string, doc *KBDocument, totalDocs int) float64 {
	if doc.TotalWords == 0 {
		return 0
	}

	score := 0.0
	for _, qword := range queryWords {
		if count, ok := doc.Words[qword]; ok {
			// TF: term frequency in doc
			tf := float64(count) / float64(doc.TotalWords)
			// IDF: inverse doc frequency (log scale)
			idf := math.Log(float64(totalDocs) / 1.0) // simplified: assume 1 doc has this word
			score += tf * idf
		}
	}
	return score
}

var KBAdd = &ToolDef{
	Name:        "kb_add",
	Description: "Add a document to the knowledge base. Can provide text directly, fetch from URL, or read from file. Automatically tokenized for search.",
	Args: []ToolArg{
		{Name: "title", Description: "Document title", Required: true},
		{Name: "content", Description: "Document text content (or omit and provide url/file)", Required: false},
		{Name: "url", Description: "Fetch document from this URL (text/HTML)", Required: false},
		{Name: "file", Description: "Read document from this file path", Required: false},
		{Name: "tags", Description: "Comma-separated tags (optional)", Required: false},
	},
	Execute: func(args map[string]string) string {
		title := strings.TrimSpace(args["title"])
		if title == "" {
			return "Error: title is required"
		}

		var content string

		// Determine content source
		if contentArg := strings.TrimSpace(args["content"]); contentArg != "" {
			content = contentArg
		} else if urlArg := strings.TrimSpace(args["url"]); urlArg != "" {
			// Fetch from URL
			resp, err := http.Get(urlArg)
			if err != nil {
				return fmt.Sprintf("Error fetching URL: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 500000)) // 500KB limit
			content = string(body)
			// Strip HTML tags (crude but effective)
			content = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(content, " ")
			content = strings.TrimSpace(content)
		} else if fileArg := strings.TrimSpace(args["file"]); fileArg != "" {
			// Read from file
			data, err := os.ReadFile(fileArg)
			if err != nil {
				return fmt.Sprintf("Error reading file: %v", err)
			}
			content = string(data)
		} else {
			return "Error: provide either content, url, or file"
		}

		if content == "" {
			return "Error: no content to store"
		}

		// Parse tags
		tagsStr := strings.TrimSpace(args["tags"])
		var tags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				tag := strings.TrimSpace(t)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}

		// Create document
		doc := &KBDocument{
			ID:        uuid.New().String(),
			Title:     title,
			Content:   content,
			Tags:      tags,
			CreatedAt: time.Now().Format(time.RFC3339),
			Words:     buildWordFreq(content),
		}
		doc.TotalWords = len(tokenizeKB(content))

		kb.mu.Lock()
		kb.docs[doc.ID] = doc
		kb.mu.Unlock()

		go persistKB()

		return fmt.Sprintf("Document added: %q (%d words, id: %s)", title, doc.TotalWords, doc.ID[:8])
	},
}

var KBSearch = &ToolDef{
	Name:        "kb_search",
	Description: "Search the knowledge base by keyword. Returns top matches ranked by TF-IDF relevance.",
	Args: []ToolArg{
		{Name: "query", Description: "Search query (keywords)", Required: true},
		{Name: "limit", Description: "Number of results (default 5, max 10)", Required: false},
	},
	Execute: func(args map[string]string) string {
		query := strings.TrimSpace(args["query"])
		if query == "" {
			return "Error: query is required"
		}

		limit := 5
		if l := strings.TrimSpace(args["limit"]); l != "" {
			n := 0
			if _, err := fmt.Sscanf(l, "%d", &n); err == nil && n > 0 {
				limit = n
				if limit > 10 {
					limit = 10
				}
			}
		}

		queryWords := tokenizeKB(query)
		if len(queryWords) == 0 {
			return "Error: query contains no searchable words"
		}

		kb.mu.Lock()
		docs := make([]*KBDocument, 0, len(kb.docs))
		for _, doc := range kb.docs {
			docs = append(docs, doc)
		}
		kb.mu.Unlock()

		if len(docs) == 0 {
			return "No documents in knowledge base."
		}

		// Score all docs
		type scoredDoc struct {
			doc   *KBDocument
			score float64
		}
		var results []scoredDoc
		for _, doc := range docs {
			score := tfidfScore(queryWords, doc, len(docs))
			if score > 0 {
				results = append(results, scoredDoc{doc, score})
			}
		}

		if len(results) == 0 {
			return fmt.Sprintf("No matches found for %q", query)
		}

		// Sort by score descending
		sort.Slice(results, func(i, j int) bool {
			return results[i].score > results[j].score
		})

		// Return top results
		if len(results) > limit {
			results = results[:limit]
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Search results for %q (%d matches):\n\n", query, len(results)))

		for i, s := range results {
			snippet := s.doc.Content
			if len(snippet) > 150 {
				snippet = snippet[:150] + "..."
			}
			snippet = strings.ReplaceAll(snippet, "\n", " ")

			sb.WriteString(fmt.Sprintf("%d. %s (score: %.2f)\n", i+1, s.doc.Title, s.score))
			if len(s.doc.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(s.doc.Tags, ", ")))
			}
			sb.WriteString(fmt.Sprintf("   %s\n", snippet))
			sb.WriteString("\n")
		}

		return strings.TrimRight(sb.String(), "\n")
	},
}

var KBList = &ToolDef{
	Name:        "kb_list",
	Description: "List all documents in the knowledge base with metadata.",
	Args:        []ToolArg{},
	Execute: func(args map[string]string) string {
		kb.mu.Lock()
		docs := make([]*KBDocument, 0, len(kb.docs))
		for _, doc := range kb.docs {
			docs = append(docs, doc)
		}
		kb.mu.Unlock()

		if len(docs) == 0 {
			return "Knowledge base is empty."
		}

		// Sort by creation date, newest first
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].CreatedAt > docs[j].CreatedAt
		})

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Knowledge Base (%d documents):\n\n", len(docs)))

		for i, doc := range docs {
			sb.WriteString(fmt.Sprintf("%d. %s (id: %s)\n", i+1, doc.Title, doc.ID[:8]))
			sb.WriteString(fmt.Sprintf("   Words: %d | Created: %s\n", doc.TotalWords, doc.CreatedAt[:10]))
			if len(doc.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(doc.Tags, ", ")))
			}
			sb.WriteString("\n")
		}

		return strings.TrimRight(sb.String(), "\n")
	},
}

var KBDelete = &ToolDef{
	Name:        "kb_delete",
	Description: "Delete a document from the knowledge base by ID.",
	Args: []ToolArg{
		{Name: "id", Description: "Document ID (can be shortened to first 8 chars)", Required: true},
	},
	Execute: func(args map[string]string) string {
		idArg := strings.TrimSpace(args["id"])
		if idArg == "" {
			return "Error: id is required"
		}

		kb.mu.Lock()
		defer kb.mu.Unlock()

		var toDelete string
		for id := range kb.docs {
			if id == idArg || strings.HasPrefix(id, idArg) {
				toDelete = id
				break
			}
		}

		if toDelete == "" {
			return fmt.Sprintf("Error: document %q not found", idArg)
		}

		title := kb.docs[toDelete].Title
		delete(kb.docs, toDelete)

		go persistKB()

		return fmt.Sprintf("Deleted: %q", title)
	},
}
