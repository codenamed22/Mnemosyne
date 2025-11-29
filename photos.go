package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

const (
	thumbnailSize = 300
)

// PhotoInfo represents metadata about a photo
type PhotoInfo struct {
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	UploadedAt  time.Time `json:"uploaded_at"`
	ThumbnailURL string   `json:"thumbnail_url"`
	OriginalURL  string   `json:"original_url"`
}

// PhotoManager handles photo operations
type PhotoManager struct {
	storagePath string
	maxUploadMB int64
}

// NewPhotoManager creates a new photo manager
func NewPhotoManager(storagePath string, maxUploadMB int64) *PhotoManager {
	return &PhotoManager{
		storagePath: storagePath,
		maxUploadMB: maxUploadMB,
	}
}

// getOriginalsPath returns the path to the originals directory
func (pm *PhotoManager) getOriginalsPath() string {
	return filepath.Join(pm.storagePath, "originals")
}

// getThumbnailsPath returns the path to the thumbnails directory
func (pm *PhotoManager) getThumbnailsPath() string {
	return filepath.Join(pm.storagePath, "thumbnails")
}

// ListPhotos returns a list of all photos
func (pm *PhotoManager) ListPhotos() ([]PhotoInfo, error) {
	originalsPath := pm.getOriginalsPath()
	
	entries, err := os.ReadDir(originalsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read photos directory: %v", err)
	}
	
	photos := make([]PhotoInfo, 0) // Initialize as empty slice, not nil
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		if !isImageFile(entry.Name()) {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		photo := PhotoInfo{
			Filename:     entry.Name(),
			Size:         info.Size(),
			UploadedAt:   info.ModTime(),
			ThumbnailURL: "/api/photos/thumbnail/" + url.PathEscape(entry.Name()),
			OriginalURL:  "/api/photos/original/" + url.PathEscape(entry.Name()),
		}
		
		photos = append(photos, photo)
	}
	
	return photos, nil
}

// SavePhoto saves an uploaded photo and generates a thumbnail
func (pm *PhotoManager) SavePhoto(filename string, data []byte) error {
	// Validate file extension
	if !isImageFile(filename) {
		return fmt.Errorf("unsupported file type")
	}
	
	// Validate magic bytes
	if _, err := validateImageMagicBytes(data); err != nil {
		return fmt.Errorf("invalid image file: %v", err)
	}
	
	// Sanitize filename
	filename = sanitizeFilename(filename)
	
	// Check if file already exists, add suffix if needed
	filename = pm.getUniqueFilename(filename)
	
	originalPath := filepath.Join(pm.getOriginalsPath(), filename)
	thumbnailPath := filepath.Join(pm.getThumbnailsPath(), filename)
	
	// Save original
	if err := os.WriteFile(originalPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save photo: %v", err)
	}
	
	// Generate thumbnail
	if err := pm.generateThumbnail(originalPath, thumbnailPath); err != nil {
		// Log error but don't fail - thumbnail generation is not critical
		fmt.Printf("Warning: failed to generate thumbnail for %s: %v\n", filename, err)
	}
	
	return nil
}

// generateThumbnail creates a thumbnail of the image
func (pm *PhotoManager) generateThumbnail(srcPath, dstPath string) error {
	// Open source image
	src, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %v", err)
	}
	
	// Create thumbnail (fit to max dimension while maintaining aspect ratio)
	thumbnail := imaging.Fit(src, thumbnailSize, thumbnailSize, imaging.Lanczos)
	
	// Save thumbnail
	if err := imaging.Save(thumbnail, dstPath); err != nil {
		return fmt.Errorf("failed to save thumbnail: %v", err)
	}
	
	return nil
}

// getUniqueFilename returns a unique filename by adding a suffix if needed
func (pm *PhotoManager) getUniqueFilename(filename string) string {
	originalPath := filepath.Join(pm.getOriginalsPath(), filename)
	
	// If file doesn't exist, return as-is
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return filename
	}
	
	// Add timestamp suffix
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	timestamp := time.Now().Unix()
	
	return fmt.Sprintf("%s_%d%s", name, timestamp, ext)
}

// GetOriginal returns the path to an original photo
func (pm *PhotoManager) GetOriginal(filename string) (string, error) {
	// Sanitize filename to prevent path traversal
	filename = filepath.Base(filename)
	
	if !isImageFile(filename) {
		return "", fmt.Errorf("invalid file type")
	}
	
	path := filepath.Join(pm.getOriginalsPath(), filename)
	
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found")
	}
	
	return path, nil
}

// GetThumbnail returns the path to a thumbnail
func (pm *PhotoManager) GetThumbnail(filename string) (string, error) {
	// Sanitize filename to prevent path traversal
	filename = filepath.Base(filename)
	
	if !isImageFile(filename) {
		return "", fmt.Errorf("invalid file type")
	}
	
	path := filepath.Join(pm.getThumbnailsPath(), filename)
	
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try to generate thumbnail from original
		originalPath, err := pm.GetOriginal(filename)
		if err != nil {
			return "", fmt.Errorf("file not found")
		}
		
		if err := pm.generateThumbnail(originalPath, path); err != nil {
			return "", fmt.Errorf("failed to generate thumbnail: %v", err)
		}
	}
	
	return path, nil
}

// DeletePhoto deletes a photo and its thumbnail
func (pm *PhotoManager) DeletePhoto(filename string) error {
	// Sanitize filename to prevent path traversal
	filename = filepath.Base(filename)
	
	if !isImageFile(filename) {
		return fmt.Errorf("invalid file type")
	}
	
	originalPath := filepath.Join(pm.getOriginalsPath(), filename)
	thumbnailPath := filepath.Join(pm.getThumbnailsPath(), filename)
	
	// Delete original
	if err := os.Remove(originalPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete photo: %v", err)
	}
	
	// Delete thumbnail (ignore errors if doesn't exist)
	os.Remove(thumbnailPath)
	
	return nil
}

// API Handler Functions

// HandleUpload handles photo upload requests
func (app *App) HandleUpload(w http.ResponseWriter, r *http.Request) {
	// Validate session
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Validate CSRF token
	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	
	// Parse multipart form
	if err := r.ParseMultipartForm(app.config.MaxUploadMB << 20); err != nil {
		http.Error(w, "Failed to parse upload", http.StatusBadRequest)
		return
	}
	
	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	// Check file size
	if header.Size > app.config.MaxUploadMB<<20 {
		http.Error(w, fmt.Sprintf("File too large (max %dMB)", app.config.MaxUploadMB), http.StatusBadRequest)
		return
	}
	
	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}
	
	// Save photo
	if err := app.photoMgr.SavePhoto(header.Filename, data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save photo: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"message": "Photo uploaded successfully",
	})
}

// HandleListPhotos handles photo listing requests
func (app *App) HandleListPhotos(w http.ResponseWriter, r *http.Request) {
	// Validate session
	if _, err := app.sessionMgr.ValidateSession(r); err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	photos, err := app.photoMgr.ListPhotos()
	if err != nil {
		http.Error(w, "Failed to list photos", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}

// HandleGetOriginal serves original photos
func (app *App) HandleGetOriginal(w http.ResponseWriter, r *http.Request) {
	// Validate session
	if _, err := app.sessionMgr.ValidateSession(r); err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	filename := r.PathValue("filename")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}
	
	path, err := app.photoMgr.GetOriginal(filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	
	http.ServeFile(w, r, path)
}

// HandleGetThumbnail serves thumbnail images
func (app *App) HandleGetThumbnail(w http.ResponseWriter, r *http.Request) {
	// Validate session
	if _, err := app.sessionMgr.ValidateSession(r); err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	filename := r.PathValue("filename")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}
	
	path, err := app.photoMgr.GetThumbnail(filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	
	http.ServeFile(w, r, path)
}

// HandleDeletePhoto handles photo deletion requests
func (app *App) HandleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	// Validate session
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Validate CSRF token
	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	
	filename := r.PathValue("filename")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}
	
	if err := app.photoMgr.DeletePhoto(filename); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete photo: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"message": "Photo deleted successfully",
	})
}

