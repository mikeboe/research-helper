package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type PdfScrapeResponsePage struct {
	Index    int    `json:"index"`
	Markdown string `json:"markdown"`
}

type OcrResponse struct {
	Pages []PdfScrapeResponsePage `json:"pages"`
}

// ScrapePDF extracts the contents of a PDF file as text using Mistral OCR API.
func ScrapePDF(url string) (string, error) {
	url = strings.Replace(url, "http://", "https://", 1)

	// Ensure env vars are loaded
	_ = godotenv.Load()

	client := &http.Client{}
	baseUrl := "https://api.mistral.ai/v1/ocr"
	apiKey := os.Getenv("MISTRAL_API_KEY")

	if apiKey == "" {
		return "", fmt.Errorf("MISTRAL_API_KEY is not set")
	}

	fmt.Printf("PDF Scraper called with URL: %s\n", url)

	reqBody := map[string]interface{}{
		"model": "mistral-ocr-latest",
		"document": map[string]string{
			"type":         "document_url",
			"document_url": url,
		},
		"include_image_base64": true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}
	clientReq, err := http.NewRequest("POST", baseUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	clientReq.Header.Set("Content-Type", "application/json")
	clientReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(clientReq)
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status: %s, body: %s", resp.Status, string(body))
	}

	var ocrResponse OcrResponse
	err = json.Unmarshal(body, &ocrResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal OCR response: %w", err)
	}

	var response string
	response += "-----\n"
	response += fmt.Sprintf("# URL: %s\n", url)
	response += "-----\n\n"
	for _, page := range ocrResponse.Pages {
		response += fmt.Sprintf("- Page %d -\n", page.Index)
		response += page.Markdown + "\n\n"
	}
	return response, nil
}
