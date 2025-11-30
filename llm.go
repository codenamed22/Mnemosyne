package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMProvider represents the supported LLM providers
type LLMProvider string

const (
	ProviderOpenAI  LLMProvider = "openai"
	ProviderAzure   LLMProvider = "azure"
	ProviderGemini  LLMProvider = "gemini"
	ProviderCustom  LLMProvider = "custom"
)

// LLMConfig contains configuration for the LLM service
type LLMConfig struct {
	Provider        LLMProvider `json:"provider"`         // openai, azure, gemini, custom
	APIKey          string      `json:"api_key"`          // API key for the provider
	BaseURL         string      `json:"base_url"`         // Base URL (for Azure/custom)
	Model           string      `json:"model"`            // Model name (e.g., gpt-4o, gemini-1.5-pro)
	AzureDeployment string      `json:"azure_deployment"` // Azure deployment name
	AzureAPIVersion string      `json:"azure_api_version"` // Azure API version
}

// LLMClient handles communication with LLM providers
type LLMClient struct {
	config     LLMConfig
	httpClient *http.Client
}

// PhotoAnalysis represents the AI analysis of a photo
type PhotoAnalysis struct {
	PhotoID     int64   `json:"photo_id"`
	Sharpness   int     `json:"sharpness"`   // 0-100
	Exposure    int     `json:"exposure"`    // 0-100
	Composition int     `json:"composition"` // 0-100
	FaceQuality int     `json:"face_quality"` // 0-100
	OverallScore int    `json:"overall_score"` // 0-100
	Issues      []string `json:"issues"`      // List of detected issues
}

// BestPhotoResult represents the result of best photo selection
type BestPhotoResult struct {
	BestPhotoID int64           `json:"best_photo_id"`
	Reasoning   string          `json:"reasoning"`
	Analyses    []PhotoAnalysis `json:"analyses"`
}

// NewLLMClient creates a new LLM client with the given configuration
func NewLLMClient(config LLMConfig) *LLMClient {
	// Set default values
	if config.Model == "" {
		switch config.Provider {
		case ProviderOpenAI, ProviderAzure:
			config.Model = "gpt-4o"
		case ProviderGemini:
			config.Model = "gemini-1.5-pro"
		}
	}

	if config.BaseURL == "" {
		switch config.Provider {
		case ProviderOpenAI:
			config.BaseURL = "https://api.openai.com/v1"
		case ProviderGemini:
			config.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
		}
	}

	if config.AzureAPIVersion == "" {
		config.AzureAPIVersion = "2024-02-15-preview"
	}

	return &LLMClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Long timeout for vision models
		},
	}
}

// SelectBestPhoto analyzes a group of photos and selects the best one
func (c *LLMClient) SelectBestPhoto(photoPaths []string, photoIDs []int64) (*BestPhotoResult, error) {
	if len(photoPaths) == 0 {
		return nil, fmt.Errorf("no photos provided")
	}

	if len(photoPaths) == 1 {
		return &BestPhotoResult{
			BestPhotoID: photoIDs[0],
			Reasoning:   "Only one photo in the group",
			Analyses:    []PhotoAnalysis{},
		}, nil
	}

	switch c.config.Provider {
	case ProviderOpenAI, ProviderAzure, ProviderCustom:
		return c.selectBestPhotoOpenAI(photoPaths, photoIDs)
	case ProviderGemini:
		return c.selectBestPhotoGemini(photoPaths, photoIDs)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", c.config.Provider)
	}
}

// selectBestPhotoOpenAI uses OpenAI/Azure/Custom API to select the best photo
func (c *LLMClient) selectBestPhotoOpenAI(photoPaths []string, photoIDs []int64) (*BestPhotoResult, error) {
	// Build the messages with images
	content := []map[string]interface{}{
		{
			"type": "text",
			"text": buildPhotoAnalysisPrompt(photoIDs),
		},
	}

	// Add each photo as an image
	for i, path := range photoPaths {
		imageData, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read image %d: %w", i+1, err)
		}

		// Determine MIME type
		mimeType := "image/jpeg"
		if strings.HasSuffix(strings.ToLower(path), ".png") {
			mimeType = "image/png"
		} else if strings.HasSuffix(strings.ToLower(path), ".webp") {
			mimeType = "image/webp"
		}

		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imageData)),
			},
		})
	}

	// Build request body
	requestBody := map[string]interface{}{
		"model": c.config.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": content,
			},
		},
		"max_tokens": 2000,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL based on provider
	var url string
	switch c.config.Provider {
	case ProviderAzure:
		url = fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
			c.config.BaseURL, c.config.AzureDeployment, c.config.AzureAPIVersion)
	default:
		url = c.config.BaseURL + "/chat/completions"
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Set authorization header based on provider
	switch c.config.Provider {
	case ProviderAzure:
		req.Header.Set("api-key", c.config.APIKey)
	default:
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// Parse the JSON response content
	return parsePhotoAnalysisResponse(apiResp.Choices[0].Message.Content, photoIDs)
}

// selectBestPhotoGemini uses Google Gemini API to select the best photo
func (c *LLMClient) selectBestPhotoGemini(photoPaths []string, photoIDs []int64) (*BestPhotoResult, error) {
	// Build parts array with prompt and images
	parts := []map[string]interface{}{
		{
			"text": buildPhotoAnalysisPrompt(photoIDs),
		},
	}

	// Add each photo as inline data
	for i, path := range photoPaths {
		imageData, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read image %d: %w", i+1, err)
		}

		// Determine MIME type
		mimeType := "image/jpeg"
		if strings.HasSuffix(strings.ToLower(path), ".png") {
			mimeType = "image/png"
		} else if strings.HasSuffix(strings.ToLower(path), ".webp") {
			mimeType = "image/webp"
		}

		parts = append(parts, map[string]interface{}{
			"inline_data": map[string]string{
				"mime_type": mimeType,
				"data":      base64.StdEncoding.EncodeToString(imageData),
			},
		})
	}

	// Build request body
	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": parts,
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"maxOutputTokens":  2000,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s",
		c.config.BaseURL, c.config.Model, c.config.APIKey)

	// Create and send request
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	return parsePhotoAnalysisResponse(apiResp.Candidates[0].Content.Parts[0].Text, photoIDs)
}

// buildPhotoAnalysisPrompt creates the prompt for photo analysis
func buildPhotoAnalysisPrompt(photoIDs []int64) string {
	photoList := ""
	for i, id := range photoIDs {
		photoList += fmt.Sprintf("- Photo %d (ID: %d)\n", i+1, id)
	}

	return fmt.Sprintf(`You are an expert photo curator. Analyze the following %d photos and determine which one is the best.

Photos to analyze:
%s

For each photo, evaluate:
1. **Sharpness/Focus** (0-100): Is the subject in focus? Is the image sharp?
2. **Exposure/Brightness** (0-100): Is the photo well-exposed? Not too dark or too bright?
3. **Composition** (0-100): Is the framing and composition pleasing?
4. **Face Quality** (0-100): If there are faces, are eyes open? Are expressions natural?

Then select the BEST photo overall and explain your reasoning.

Respond in this exact JSON format:
{
  "best_photo_id": <the ID of the best photo>,
  "reasoning": "<1-2 sentences explaining why this photo is the best>",
  "analyses": [
    {
      "photo_id": <photo ID>,
      "sharpness": <0-100>,
      "exposure": <0-100>,
      "composition": <0-100>,
      "face_quality": <0-100>,
      "overall_score": <0-100>,
      "issues": ["<issue1>", "<issue2>"]
    }
  ]
}`, len(photoIDs), photoList)
}

// parsePhotoAnalysisResponse parses the LLM response into a structured result
func parsePhotoAnalysisResponse(content string, photoIDs []int64) (*BestPhotoResult, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)
	
	// Handle markdown code blocks
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		content = strings.Join(jsonLines, "\n")
	}

	var result BestPhotoResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w\nContent: %s", err, content)
	}

	// Validate best_photo_id is in our list
	validID := false
	for _, id := range photoIDs {
		if id == result.BestPhotoID {
			validID = true
			break
		}
	}

	if !validID && len(photoIDs) > 0 {
		// Default to first photo if LLM gave invalid ID
		result.BestPhotoID = photoIDs[0]
		result.Reasoning = "Selected first photo (LLM response was invalid)"
	}

	return &result, nil
}

// IsConfigured checks if the LLM client has valid configuration
func (c *LLMClient) IsConfigured() bool {
	return c.config.APIKey != "" && c.config.Provider != ""
}

// GetProvider returns the configured provider
func (c *LLMClient) GetProvider() LLMProvider {
	return c.config.Provider
}

