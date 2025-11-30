package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"
)

// EmbeddingService handles communication with the CLIP embedding service
type EmbeddingService struct {
	baseURL    string
	httpClient *http.Client
}

// EmbeddingRequest is the request to generate an embedding
type EmbeddingRequest struct {
	ImageBase64 string `json:"image_base64"`
	ImageID     string `json:"image_id,omitempty"`
}

// EmbeddingResponse is the response from the embedding service
type EmbeddingResponse struct {
	ImageID   string    `json:"image_id"`
	Embedding []float64 `json:"embedding"`
	Dimension int       `json:"dimension"`
}

// BatchEmbeddingRequest is a batch request for multiple embeddings
type BatchEmbeddingRequest struct {
	Images []EmbeddingRequest `json:"images"`
}

// BatchEmbeddingResponse is the response for batch embeddings
type BatchEmbeddingResponse struct {
	Embeddings []EmbeddingResponse `json:"embeddings"`
}

// HealthResponse is the health check response
type HealthResponse struct {
	Status      string `json:"status"`
	ModelLoaded bool   `json:"model_loaded"`
	Device      string `json:"device"`
}

// NewEmbeddingService creates a new embedding service client
func NewEmbeddingService(baseURL string) *EmbeddingService {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8081"
	}
	return &EmbeddingService{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for model inference
		},
	}
}

// IsHealthy checks if the embedding service is running and ready
func (es *EmbeddingService) IsHealthy() (bool, error) {
	resp, err := es.httpClient.Get(es.baseURL + "/health")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false, err
	}

	return health.Status == "healthy" && health.ModelLoaded, nil
}

// GenerateEmbedding generates an embedding for a single image file
func (es *EmbeddingService) GenerateEmbedding(imagePath string, imageID string) ([]float64, error) {
	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	// Encode to base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	// Create request
	req := EmbeddingRequest{
		ImageBase64: imageBase64,
		ImageID:     imageID,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	resp, err := es.httpClient.Post(
		es.baseURL+"/embed",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding service error: %s", string(body))
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return embResp.Embedding, nil
}

// GenerateEmbeddingFromBytes generates an embedding from image bytes
func (es *EmbeddingService) GenerateEmbeddingFromBytes(imageData []byte, imageID string) ([]float64, error) {
	// Encode to base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	// Create request
	req := EmbeddingRequest{
		ImageBase64: imageBase64,
		ImageID:     imageID,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	resp, err := es.httpClient.Post(
		es.baseURL+"/embed",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding service error: %s", string(body))
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return embResp.Embedding, nil
}

// CosineSimilarity calculates the cosine similarity between two embedding vectors
// Returns a value between 0 and 1, where 1 means identical
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	// Cosine similarity formula
	similarity := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))

	// Clamp to [0, 1] (embeddings are normalized, so this handles floating point errors)
	if similarity < 0 {
		similarity = 0
	} else if similarity > 1 {
		similarity = 1
	}

	return similarity
}

// CosineDistance calculates the cosine distance (1 - similarity) between two embeddings
// Returns a value between 0 and 1, where 0 means identical
func CosineDistance(a, b []float64) float64 {
	return 1 - CosineSimilarity(a, b)
}

// PhotoSimilarity represents the similarity between two photos
type PhotoSimilarity struct {
	PhotoID1   int64   `json:"photo_id_1"`
	PhotoID2   int64   `json:"photo_id_2"`
	Similarity float64 `json:"similarity"` // 0 to 1
}

// FindSimilarPhotos finds all pairs of photos with similarity above the threshold
func FindSimilarPhotos(embeddings map[int64][]float64, threshold float64) []PhotoSimilarity {
	var similarities []PhotoSimilarity

	// Get all photo IDs
	ids := make([]int64, 0, len(embeddings))
	for id := range embeddings {
		ids = append(ids, id)
	}

	// Compare all pairs
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			id1, id2 := ids[i], ids[j]
			sim := CosineSimilarity(embeddings[id1], embeddings[id2])

			if sim >= threshold {
				similarities = append(similarities, PhotoSimilarity{
					PhotoID1:   id1,
					PhotoID2:   id2,
					Similarity: sim,
				})
			}
		}
	}

	return similarities
}

// EmbeddingToBytes converts an embedding to bytes for database storage
func EmbeddingToBytes(embedding []float64) []byte {
	data, _ := json.Marshal(embedding)
	return data
}

// EmbeddingFromBytes converts bytes from database to embedding
func EmbeddingFromBytes(data []byte) ([]float64, error) {
	var embedding []float64
	err := json.Unmarshal(data, &embedding)
	return embedding, err
}

