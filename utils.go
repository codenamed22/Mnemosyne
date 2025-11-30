package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// generateRandomPassword creates a cryptographically secure random password
// Falls back to a timestamp-based password if crypto fails (unlikely)
func generateRandomPassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback - should never happen in practice
		return fmt.Sprintf("fallback_%d", time.Now().UnixNano())[:length]
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// generateRandomToken creates a cryptographically secure random token
func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// Reserved Windows filenames that cannot be used
var reservedWindowsNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// sanitizeFilename removes dangerous characters from filenames
func sanitizeFilename(filename string) string {
	// Get extension
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// URL decode to handle %2f and similar encoded characters
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	
	// Remove path separators and other dangerous chars
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
		"\x00", "_",
		"<", "_",
		">", "_",
		":", "_",
		"\"", "_",
		"|", "_",
		"?", "_",
		"*", "_",
	)
	name = replacer.Replace(name)

	// Remove leading dots (hidden files on Unix)
	name = strings.TrimLeft(name, ".")

	// Check for reserved Windows filenames
	upperName := strings.ToUpper(name)
	if reservedWindowsNames[upperName] {
		name = "_" + name
	}

	// Also check with extension stripped (e.g., "CON.txt" is also reserved)
	nameWithoutExt := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	if reservedWindowsNames[nameWithoutExt] {
		name = "_" + name
	}

	// Ensure name is not empty after sanitization
	if name == "" {
		name = "unnamed"
	}
	
	// Limit length
	if len(name) > MaxFilenameLength {
		name = name[:MaxFilenameLength]
	}
	
	return name + ext
}

// isImageFile checks if the file extension is an allowed image type
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	allowed := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
	return allowed[ext]
}

// validateImageMagicBytes checks if the file content matches image type
func validateImageMagicBytes(data []byte) (string, error) {
	if len(data) < 12 {
		return "", fmt.Errorf("file too small")
	}
	
	// JPEG
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", nil
	}
	
	// PNG
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png", nil
	}
	
	// GIF
	if len(data) >= 6 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif", nil
	}
	
	// WebP
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp", nil
	}
	
	return "", fmt.Errorf("unsupported image format")
}

