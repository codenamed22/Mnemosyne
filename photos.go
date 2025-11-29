package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/disintegration/imaging"
)

const (
	thumbnailSize = 300
)

// PhotoManager handles photo operations
type PhotoManager struct {
	storagePath string
	maxUploadMB int64
	db          *Database
}

// NewPhotoManager creates a new photo manager
func NewPhotoManager(storagePath string, maxUploadMB int64, db *Database) *PhotoManager {
	return &PhotoManager{
		storagePath: storagePath,
		maxUploadMB: maxUploadMB,
		db:          db,
	}
}

// getUserPath returns the storage path for a specific user
func (pm *PhotoManager) getUserPath(userID int64) string {
	return filepath.Join(pm.storagePath, "users", fmt.Sprintf("%d", userID))
}

// getOriginalsPath returns the path to originals for a user
func (pm *PhotoManager) getOriginalsPath(userID int64) string {
	return filepath.Join(pm.getUserPath(userID), "originals")
}

// getThumbnailsPath returns the path to thumbnails for a user
func (pm *PhotoManager) getThumbnailsPath(userID int64) string {
	return filepath.Join(pm.getUserPath(userID), "thumbnails")
}

// EnsureUserDirectories creates storage directories for a user
func (pm *PhotoManager) EnsureUserDirectories(userID int64) error {
	dirs := []string{
		pm.getOriginalsPath(userID),
		pm.getThumbnailsPath(userID),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	return nil
}

// SavePhoto saves an uploaded photo for a user
func (pm *PhotoManager) SavePhoto(filename string, data []byte, userID int64) (*Photo, error) {
	// Validate file extension
	if !isImageFile(filename) {
		return nil, fmt.Errorf("unsupported file type")
	}

	// Validate magic bytes
	if _, err := validateImageMagicBytes(data); err != nil {
		return nil, fmt.Errorf("invalid image file: %v", err)
	}

	// Sanitize filename
	filename = sanitizeFilename(filename)

	// Ensure user directories exist
	if err := pm.EnsureUserDirectories(userID); err != nil {
		return nil, err
	}

	// Check if file already exists, add suffix if needed
	filename = pm.getUniqueFilename(filename, userID)

	originalPath := filepath.Join(pm.getOriginalsPath(userID), filename)
	thumbnailPath := filepath.Join(pm.getThumbnailsPath(userID), filename)

	// Save original
	if err := os.WriteFile(originalPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to save photo: %v", err)
	}

	// Generate thumbnail
	if err := pm.generateThumbnail(originalPath, thumbnailPath); err != nil {
		fmt.Printf("Warning: failed to generate thumbnail for %s: %v\n", filename, err)
	}

	// Save to database
	photo, err := pm.db.CreatePhoto(filename, userID, int64(len(data)))
	if err != nil {
		// Clean up files if database save fails
		os.Remove(originalPath)
		os.Remove(thumbnailPath)
		return nil, err
	}

	return photo, nil
}

// generateThumbnail creates a thumbnail of the image
func (pm *PhotoManager) generateThumbnail(srcPath, dstPath string) error {
	src, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %v", err)
	}

	thumbnail := imaging.Fit(src, thumbnailSize, thumbnailSize, imaging.Lanczos)

	if err := imaging.Save(thumbnail, dstPath); err != nil {
		return fmt.Errorf("failed to save thumbnail: %v", err)
	}

	return nil
}

// getUniqueFilename returns a unique filename for a user
func (pm *PhotoManager) getUniqueFilename(filename string, userID int64) string {
	originalPath := filepath.Join(pm.getOriginalsPath(userID), filename)

	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return filename
	}

	// Add counter suffix
	ext := filepath.Ext(filename)
	name := filename[:len(filename)-len(ext)]

	for i := 1; i < 10000; i++ {
		newFilename := fmt.Sprintf("%s_%d%s", name, i, ext)
		newPath := filepath.Join(pm.getOriginalsPath(userID), newFilename)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newFilename
		}
	}

	return filename
}

// GetOriginalPath returns the path to an original photo
func (pm *PhotoManager) GetOriginalPath(photo *Photo) (string, error) {
	path := filepath.Join(pm.getOriginalsPath(photo.UserID), photo.Filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found")
	}

	return path, nil
}

// GetThumbnailPath returns the path to a thumbnail
func (pm *PhotoManager) GetThumbnailPath(photo *Photo) (string, error) {
	path := filepath.Join(pm.getThumbnailsPath(photo.UserID), photo.Filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try to regenerate thumbnail
		originalPath, err := pm.GetOriginalPath(photo)
		if err != nil {
			return "", fmt.Errorf("file not found")
		}

		if err := pm.generateThumbnail(originalPath, path); err != nil {
			return "", fmt.Errorf("failed to generate thumbnail: %v", err)
		}
	}

	return path, nil
}

// DeletePhoto deletes a photo and its files
func (pm *PhotoManager) DeletePhoto(photo *Photo) error {
	originalPath := filepath.Join(pm.getOriginalsPath(photo.UserID), photo.Filename)
	thumbnailPath := filepath.Join(pm.getThumbnailsPath(photo.UserID), photo.Filename)

	// Delete from database first
	if err := pm.db.DeletePhoto(photo.ID); err != nil {
		return fmt.Errorf("failed to delete photo record: %v", err)
	}

	// Delete files
	os.Remove(originalPath)
	os.Remove(thumbnailPath)

	return nil
}

// BuildPhotoURLs adds URL fields to a photo
func (pm *PhotoManager) BuildPhotoURLs(photo *Photo) {
	photo.ThumbnailURL = fmt.Sprintf("/api/photos/thumbnail/%d/%s", photo.UserID, url.PathEscape(photo.Filename))
	photo.OriginalURL = fmt.Sprintf("/api/photos/original/%d/%s", photo.UserID, url.PathEscape(photo.Filename))
}

// API Handlers

// HandleUpload handles photo upload requests
func (app *App) HandleUpload(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

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

	if header.Size > app.config.MaxUploadMB<<20 {
		http.Error(w, fmt.Sprintf("File too large (max %dMB)", app.config.MaxUploadMB), http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	photo, err := app.photoMgr.SavePhoto(header.Filename, data, session.UserID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save photo: %v", err), http.StatusInternalServerError)
		return
	}

	app.photoMgr.BuildPhotoURLs(photo)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Photo uploaded successfully",
		"photo":   photo,
	})
}

// HandleListMyPhotos lists photos for the current user
func (app *App) HandleListMyPhotos(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	photos, err := app.db.GetPhotosByUser(session.UserID)
	if err != nil {
		http.Error(w, "Failed to list photos", http.StatusInternalServerError)
		return
	}

	for _, photo := range photos {
		app.photoMgr.BuildPhotoURLs(photo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}

// HandleListSharedPhotos lists photos in the family area
func (app *App) HandleListSharedPhotos(w http.ResponseWriter, r *http.Request) {
	_, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	photos, err := app.db.GetSharedPhotos()
	if err != nil {
		http.Error(w, "Failed to list photos", http.StatusInternalServerError)
		return
	}

	for _, photo := range photos {
		app.photoMgr.BuildPhotoURLs(photo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}

// HandleListAllPhotos lists all photos (admin only)
func (app *App) HandleListAllPhotos(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	photos, err := app.db.GetAllPhotos()
	if err != nil {
		http.Error(w, "Failed to list photos", http.StatusInternalServerError)
		return
	}

	for _, photo := range photos {
		app.photoMgr.BuildPhotoURLs(photo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}

// HandleGetOriginal serves original photos
func (app *App) HandleGetOriginal(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userIDStr := r.PathValue("userID")
	filename := r.PathValue("filename")

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get photo from database
	photo, err := app.db.GetPhotoByFilename(filename, userID)
	if err != nil || photo == nil {
		http.NotFound(w, r)
		return
	}

	// Check access: owner, shared, or admin
	if photo.UserID != session.UserID && !photo.IsShared && !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path, err := app.photoMgr.GetOriginalPath(photo)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, path)
}

// HandleGetThumbnail serves thumbnail images
func (app *App) HandleGetThumbnail(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userIDStr := r.PathValue("userID")
	filename := r.PathValue("filename")

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get photo from database
	photo, err := app.db.GetPhotoByFilename(filename, userID)
	if err != nil || photo == nil {
		http.NotFound(w, r)
		return
	}

	// Check access: owner, shared, or admin
	if photo.UserID != session.UserID && !photo.IsShared && !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path, err := app.photoMgr.GetThumbnailPath(photo)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, path)
}

// HandleDeletePhoto handles photo deletion
func (app *App) HandleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	photoIDStr := r.PathValue("photoID")
	photoID, err := strconv.ParseInt(photoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid photo ID", http.StatusBadRequest)
		return
	}

	photo, err := app.db.GetPhotoByID(photoID)
	if err != nil || photo == nil {
		http.NotFound(w, r)
		return
	}

	// Check access: owner or admin
	if photo.UserID != session.UserID && !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := app.photoMgr.DeletePhoto(photo); err != nil {
		http.Error(w, "Failed to delete photo", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Photo deleted successfully",
	})
}

// HandleSharePhoto toggles photo sharing
func (app *App) HandleSharePhoto(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	photoIDStr := r.PathValue("photoID")
	photoID, err := strconv.ParseInt(photoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid photo ID", http.StatusBadRequest)
		return
	}

	photo, err := app.db.GetPhotoByID(photoID)
	if err != nil || photo == nil {
		http.NotFound(w, r)
		return
	}

	// Only owner can share/unshare (admin can't share others' photos)
	if photo.UserID != session.UserID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Toggle shared status
	newShared := !photo.IsShared
	if err := app.db.SetPhotoShared(photoID, newShared); err != nil {
		http.Error(w, "Failed to update photo", http.StatusInternalServerError)
		return
	}

	status := "unshared from"
	if newShared {
		status = "shared to"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Photo %s family area", status),
		"is_shared": newShared,
	})
}

// BulkRequest represents a request with multiple photo IDs
type BulkRequest struct {
	PhotoIDs []int64 `json:"photo_ids"`
	Share    bool    `json:"share"` // For bulk share: true = share, false = unshare
}

// HandleBulkShare shares or unshares multiple photos at once
func (app *App) HandleBulkShare(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req BulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.PhotoIDs) == 0 {
		http.Error(w, "No photos selected", http.StatusBadRequest)
		return
	}

	updated := 0
	for _, photoID := range req.PhotoIDs {
		photo, err := app.db.GetPhotoByID(photoID)
		if err != nil || photo == nil {
			continue
		}

		// Only owner can share their photos
		if photo.UserID != session.UserID {
			continue
		}

		if err := app.db.SetPhotoShared(photoID, req.Share); err != nil {
			continue
		}
		updated++
	}

	action := "unshared"
	if req.Share {
		action = "shared"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("%d photo(s) %s", updated, action),
		"updated": updated,
	})
}

// HandleBulkDownload creates a zip file with multiple photos
func (app *App) HandleBulkDownload(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req BulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.PhotoIDs) == 0 {
		http.Error(w, "No photos selected", http.StatusBadRequest)
		return
	}

	// Collect valid photos
	var photos []*Photo
	for _, photoID := range req.PhotoIDs {
		photo, err := app.db.GetPhotoByID(photoID)
		if err != nil || photo == nil {
			continue
		}

		// Check access: owner, shared, or admin
		if photo.UserID != session.UserID && !photo.IsShared && !session.IsAdmin() {
			continue
		}

		photos = append(photos, photo)
	}

	if len(photos) == 0 {
		http.Error(w, "No accessible photos", http.StatusBadRequest)
		return
	}

	// Set headers for zip download
	timestamp := time.Now().Format("2006-01-02_150405")
	filename := fmt.Sprintf("mnemosyne_photos_%s.zip", timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Create zip writer
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Add each photo to the zip
	usedNames := make(map[string]int)
	for _, photo := range photos {
		path, err := app.photoMgr.GetOriginalPath(photo)
		if err != nil {
			continue
		}

		// Handle duplicate filenames
		name := photo.Filename
		if count, exists := usedNames[name]; exists {
			ext := filepath.Ext(name)
			base := name[:len(name)-len(ext)]
			name = fmt.Sprintf("%s_%d%s", base, count+1, ext)
		}
		usedNames[photo.Filename]++

		// Create zip entry
		zipEntry, err := zipWriter.Create(name)
		if err != nil {
			continue
		}

		// Read and write file
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		_, err = io.Copy(zipEntry, file)
		file.Close()
		if err != nil {
			continue
		}
	}
}

// HandleBulkDelete deletes multiple photos at once
func (app *App) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req BulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.PhotoIDs) == 0 {
		http.Error(w, "No photos selected", http.StatusBadRequest)
		return
	}

	deleted := 0
	for _, photoID := range req.PhotoIDs {
		photo, err := app.db.GetPhotoByID(photoID)
		if err != nil || photo == nil {
			continue
		}

		// Check access: owner or admin
		if photo.UserID != session.UserID && !session.IsAdmin() {
			continue
		}

		if err := app.photoMgr.DeletePhoto(photo); err != nil {
			continue
		}
		deleted++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("%d photo(s) deleted", deleted),
		"deleted": deleted,
	})
}
