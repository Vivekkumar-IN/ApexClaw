package tools

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type gnewsFeed struct {
	Channel struct {
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			PubDate string `xml:"pubDate"`
			Source  struct {
				Value string `xml:",chardata"`
			} `xml:"source"`
		} `xml:"item"`
	} `xml:"channel"`
}

var NewsHeadlines = &ToolDef{
	Name:        "news_headlines",
	Description: "Fetch top news headlines. Optionally filter by topic (e.g. 'technology', 'sports', 'india', 'world', 'business', 'science'). Uses Google News RSS — no API key needed.",
	Args: []ToolArg{
		{Name: "topic", Description: "News topic to search (e.g. 'technology', 'india', 'cricket', or any keyword). Leave empty for top headlines.", Required: false},
		{Name: "count", Description: "Number of headlines to return (default 10, max 20)", Required: false},
		{Name: "lang", Description: "Language code (default 'en')", Required: false},
	},
	Execute: func(args map[string]string) string {
		topic := strings.TrimSpace(args["topic"])
		count := 10
		if v := strings.TrimSpace(args["count"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				count = n
				if count > 20 {
					count = 20
				}
			}
		}
		lang := strings.TrimSpace(args["lang"])
		if lang == "" {
			lang = "en"
		}

		var feedURL string
		if topic == "" {
			feedURL = fmt.Sprintf("https://news.google.com/rss?hl=%s&gl=IN&ceid=IN:%s", lang, strings.ToUpper(lang))
		} else {
			feedURL = fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=%s&gl=IN&ceid=IN:%s",
				url.QueryEscape(topic), lang, strings.ToUpper(lang))
		}

		req, _ := http.NewRequest("GET", feedURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RSS reader)")
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("Error fetching news: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var feed gnewsFeed
		if err := xml.Unmarshal(body, &feed); err != nil {
			return fmt.Sprintf("Error parsing news feed: %v", err)
		}

		items := feed.Channel.Items
		if len(items) == 0 {
			return "No news found."
		}
		if len(items) > count {
			items = items[:count]
		}

		header := "📰 Top Headlines"
		if topic != "" {
			header = fmt.Sprintf("📰 News: %s", topic)
		}
		var sb strings.Builder
		sb.WriteString(header + "\n\n")
		for i, item := range items {
			title := cleanNewsTitle(item.Title)
			source := item.Source.Value
			sb.WriteString(fmt.Sprintf("%d. %s", i+1, title))
			if source != "" {
				sb.WriteString(fmt.Sprintf(" — %s", source))
			}
			sb.WriteString("\n")
		}
		return strings.TrimRight(sb.String(), "\n")
	},
}

func cleanNewsTitle(title string) string {
	if idx := strings.LastIndex(title, " - "); idx > 0 {
		return title[:idx]
	}
	return title
}

type redditListing struct {
	Data struct {
		Children []struct {
			Data struct {
				Title       string `json:"title"`
				Author      string `json:"author"`
				Score       int    `json:"score"`
				NumComments int    `json:"num_comments"`
				URL         string `json:"url"`
				Selftext    string `json:"selftext"`
				IsSelf      bool   `json:"is_self"`
				Permalink   string `json:"permalink"`
				Ups         int    `json:"ups"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

var RedditFeed = &ToolDef{
	Name:        "reddit_feed",
	Description: "Get top posts from any subreddit or Reddit's front page. Great for trending topics, memes, news, programming, etc.",
	Args: []ToolArg{
		{Name: "subreddit", Description: "Subreddit name without 'r/' (e.g. 'technology', 'worldnews', 'india'). Leave empty for Reddit front page.", Required: false},
		{Name: "sort", Description: "Sort by: 'hot' (default), 'new', 'top', 'rising'", Required: false},
		{Name: "count", Description: "Number of posts (default 5, max 15)", Required: false},
		{Name: "time_filter", Description: "For sort=top: 'hour', 'day', 'week', 'month', 'year', 'all' (default 'day')", Required: false},
	},
	Execute: func(args map[string]string) string {
		subreddit := strings.TrimSpace(args["subreddit"])
		sort := strings.TrimSpace(args["sort"])
		if sort == "" {
			sort = "hot"
		}
		count := 5
		if v := strings.TrimSpace(args["count"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				count = n
				if count > 15 {
					count = 15
				}
			}
		}
		timeFilter := strings.TrimSpace(args["time_filter"])
		if timeFilter == "" {
			timeFilter = "day"
		}

		var apiURL string
		if subreddit == "" {
			apiURL = fmt.Sprintf("https://www.reddit.com/%s.json?limit=%d&t=%s", sort, count, timeFilter)
		} else {
			apiURL = fmt.Sprintf("https://www.reddit.com/r/%s/%s.json?limit=%d&t=%s", subreddit, sort, count, timeFilter)
		}

		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("User-Agent", "ApexClaw-Bot/1.0")
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("Error fetching Reddit: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			return fmt.Sprintf("Subreddit r/%s not found.", subreddit)
		}
		if resp.StatusCode != 200 {
			return fmt.Sprintf("Reddit API returned %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var listing redditListing
		if err := json.Unmarshal(body, &listing); err != nil {
			return fmt.Sprintf("Error parsing Reddit response: %v", err)
		}

		posts := listing.Data.Children
		if len(posts) == 0 {
			return "No posts found."
		}

		header := fmt.Sprintf("🟠 Reddit r/frontpage — %s", sort)
		if subreddit != "" {
			header = fmt.Sprintf("🟠 r/%s — %s", subreddit, sort)
		}
		var sb strings.Builder
		sb.WriteString(header + "\n\n")
		for i, p := range posts {
			d := p.Data
			title := d.Title
			if len(title) > 100 {
				title = title[:97] + "…"
			}
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
			sb.WriteString(fmt.Sprintf("   ⬆ %d  💬 %d  by u/%s\n", d.Score, d.NumComments, d.Author))
			if !d.IsSelf {
				sb.WriteString(fmt.Sprintf("   %s\n", d.URL))
			} else if len(d.Selftext) > 0 {
				snippet := strings.TrimSpace(d.Selftext)
				if len(snippet) > 120 {
					snippet = snippet[:120] + "…"
				}
				sb.WriteString(fmt.Sprintf("   %s\n", snippet))
			}
			sb.WriteString("\n")
		}
		return strings.TrimRight(sb.String(), "\n")
	},
}

type ytSearchResp struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title        string `json:"title"`
			ChannelTitle string `json:"channelTitle"`
			Description  string `json:"description"`
			PublishedAt  string `json:"publishedAt"`
		} `json:"snippet"`
	} `json:"items"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

var YouTubeSearch = &ToolDef{
	Name:        "youtube_search",
	Description: "Search YouTube for videos. Returns video titles, channel names, and links. Requires YOUTUBE_API_KEY env var (free quota: 100 searches/day).",
	Args: []ToolArg{
		{Name: "query", Description: "Search term (e.g. 'lo-fi music', 'python tutorial')", Required: true},
		{Name: "count", Description: "Number of results (default 5, max 10)", Required: false},
		{Name: "type", Description: "Result type: 'video' (default), 'channel', 'playlist'", Required: false},
	},
	Execute: func(args map[string]string) string {
		apiKey := os.Getenv("YOUTUBE_API_KEY")
		if apiKey == "" {
			return "Error: YOUTUBE_API_KEY environment variable not set. Get a free key at console.cloud.google.com"
		}
		query := strings.TrimSpace(args["query"])
		if query == "" {
			return "Error: query is required"
		}
		count := 5
		if v := strings.TrimSpace(args["count"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				count = n
				if count > 10 {
					count = 10
				}
			}
		}
		resultType := strings.TrimSpace(args["type"])
		if resultType == "" {
			resultType = "video"
		}

		apiURL := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/search?part=snippet&q=%s&maxResults=%d&type=%s&key=%s",
			url.QueryEscape(query), count, resultType, apiKey,
		)

		resp, err := http.Get(apiURL)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var result ytSearchResp
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Sprintf("Error parsing YouTube response: %v", err)
		}
		if result.Error != nil {
			return fmt.Sprintf("YouTube API error: %s", result.Error.Message)
		}
		if len(result.Items) == 0 {
			return "No results found."
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("▶️ YouTube: %s\n\n", query))
		for i, item := range result.Items {
			title := item.Snippet.Title
			channel := item.Snippet.ChannelTitle
			videoID := item.ID.VideoID
			link := ""
			if videoID != "" {
				link = fmt.Sprintf("\n   https://youtu.be/%s", videoID)
			}
			sb.WriteString(fmt.Sprintf("%d. %s\n   by %s%s\n\n", i+1, title, channel, link))
		}
		return strings.TrimRight(sb.String(), "\n")
	},
}

type redditComment struct {
	Data struct {
		Author  string `json:"author"`
		Body    string `json:"body"`
		Score   int    `json:"score"`
		Created int64  `json:"created_utc"`
	} `json:"data"`
	Kind string `json:"kind"`
}

type redditThread struct {
	Data struct {
		Children []struct {
			Data struct {
				Title       string `json:"title"`
				Author      string `json:"author"`
				Score       int    `json:"score"`
				Selftext    string `json:"selftext"`
				Permalink   string `json:"permalink"`
				CreatedUtc  int64  `json:"created_utc"`
				NumComments int    `json:"num_comments"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

var RedditThread = &ToolDef{
	Name:        "reddit_thread",
	Description: "Fetch a Reddit thread and its top comments. Provide the post URL or Reddit permalink.",
	Args: []ToolArg{
		{Name: "url", Description: "Reddit post URL or permalink (e.g. https://www.reddit.com/r/programming/comments/abc123/title/)", Required: true},
		{Name: "sort", Description: "Sort comments by: 'top' (default), 'new', 'best', 'controversial'", Required: false},
		{Name: "limit", Description: "Number of top-level comments (default 5, max 15)", Required: false},
	},
	Execute: func(args map[string]string) string {
		urlArg := strings.TrimSpace(args["url"])
		if urlArg == "" {
			return "Error: url is required"
		}

		// Clean URL: remove trailing slash, ensure .json
		urlArg = strings.TrimRight(urlArg, "/")
		if !strings.HasSuffix(urlArg, ".json") {
			urlArg += ".json"
		}

		sort := "top"
		if s := strings.TrimSpace(args["sort"]); s != "" {
			sort = strings.ToLower(s)
		}

		limit := 5
		if l := strings.TrimSpace(args["limit"]); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
				if limit > 15 {
					limit = 15
				}
			}
		}

		// Fetch thread JSON
		fullURL := urlArg
		if !strings.HasPrefix(fullURL, "http") {
			fullURL = "https://www.reddit.com" + urlArg
		}
		fullURL = strings.ReplaceAll(fullURL, ".json.json", ".json")
		fullURL += "?sort=" + sort

		req, _ := http.NewRequest("GET", fullURL, nil)
		req.Header.Set("User-Agent", "ApexClaw-Bot/1.0")
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("Error fetching thread: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Sprintf("Reddit API returned %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)

		// Parse 2-element array [post, comments_listing]
		var threadArray []interface{}
		if err := json.Unmarshal(body, &threadArray); err != nil {
			return fmt.Sprintf("Error parsing thread: %v", err)
		}

		if len(threadArray) < 2 {
			return "Error: invalid thread data"
		}

		// Extract post data
		var postTitle, postAuthor string
		var postScore, postCommentCount int
		var postBody string

		if postObj, ok := threadArray[0].(map[string]interface{}); ok {
			if dataObj, ok := postObj["data"].(map[string]interface{}); ok {
				if t, ok := dataObj["title"].(string); ok {
					postTitle = t
				}
				if a, ok := dataObj["author"].(string); ok {
					postAuthor = a
				}
				if s, ok := dataObj["score"].(float64); ok {
					postScore = int(s)
				}
				if c, ok := dataObj["num_comments"].(float64); ok {
					postCommentCount = int(c)
				}
				if b, ok := dataObj["selftext"].(string); ok {
					postBody = b
				}
			}
		}

		// Extract comments
		var comments []struct {
			author string
			score  int
			body   string
		}

		if commentsObj, ok := threadArray[1].(map[string]interface{}); ok {
			if dataObj, ok := commentsObj["data"].(map[string]interface{}); ok {
				if children, ok := dataObj["children"].([]interface{}); ok {
					for _, child := range children {
						if childObj, ok := child.(map[string]interface{}); ok {
							if dataChild, ok := childObj["data"].(map[string]interface{}); ok {
								if author, ok := dataChild["author"].(string); ok {
									if score, ok := dataChild["score"].(float64); ok {
										if body, ok := dataChild["body"].(string); ok {
											if body != "[deleted]" && body != "[removed]" && author != "[deleted]" {
												comments = append(comments, struct {
													author string
													score  int
													body   string
												}{author, int(score), body})
											}
										}
									}
								}
							}
						}
						if len(comments) >= limit {
							break
						}
					}
				}
			}
		}

		// Format output
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Thread: %s\n", postTitle))
		sb.WriteString(fmt.Sprintf("by u/%s | %d upvotes | %d comments\n\n", postAuthor, postScore, postCommentCount))

		if postBody != "" {
			snippet := strings.TrimSpace(postBody)
			if len(snippet) > 300 {
				snippet = snippet[:300] + "…"
			}
			sb.WriteString(fmt.Sprintf("Post: %s\n\n", snippet))
		}

		sb.WriteString(fmt.Sprintf("Top %d Comments (%s):\n\n", len(comments), sort))
		for i, c := range comments {
			body := c.body
			if len(body) > 200 {
				body = body[:200] + "…"
			}
			body = strings.ReplaceAll(body, "\n", " ")
			sb.WriteString(fmt.Sprintf("%d. u/%s (%d↑)\n   %s\n\n", i+1, c.author, c.score, body))
		}

		return strings.TrimRight(sb.String(), "\n")
	},
}
