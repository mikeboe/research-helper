package tools

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

// ArxivEntry struct to hold arXiv entry data
type ArxivEntry struct {
	Title     string      `xml:"title"`
	Summary   string      `xml:"summary"`
	Published string      `xml:"published"`
	Link      []ArxivLink `xml:"link"`
}

// ArxivLink struct to hold arXiv link data
type ArxivLink struct {
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

// ArxivFeed struct to hold the entire arXiv feed
type ArxivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entry   []ArxivEntry `xml:"entry"`
}

// SearchArxiv queries the Arxiv API and returns a formatted string of results.
func SearchArxiv(query string, maxResults int) (string, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	fmt.Printf("Searching arXiv for query: %s, max results: %d\n", query, maxResults)

	// Construct the arXiv API URL
	baseURL := "https://export.arxiv.org/api/query?"
	params := url.Values{}
	params.Add("search_query", query)
	params.Add("max_results", strconv.Itoa(maxResults))
	params.Add("start", "0") // Start from the first result

	apiURL := baseURL + params.Encode()

	// Make the API request
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("API request made", "url", apiURL)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("API returned non-200 status code", "status", resp.StatusCode, "body", string(bodyBytes))
		return "", fmt.Errorf("API returned non-200 status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Info("API response received", "status", resp.StatusCode)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Info("API response body read", "size", len(body))

	// Unmarshal the XML response
	var feed ArxivFeed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal XML: %w", err)
	}

	// Format the response
	var response string
	for _, entry := range feed.Entry {
		response += fmt.Sprintf("# Title: %s\n", entry.Title)
		response += fmt.Sprintf("## Summary: %s\n", entry.Summary)
		response += fmt.Sprintf("## Published: %s\n", entry.Published)
		for _, link := range entry.Link {
			if link.Type == "application/pdf" {
				response += fmt.Sprintf("## PDF Link: %s\n", link.Href)
				break
			}
		}
		response += "\n"
	}

	if response == "" {
		response = "No results found for query: " + query
	}

	return response, nil
}
