package main

// Application constants
// Centralizing magic numbers for maintainability and clarity

const (
	// Security
	BcryptCost          = 12        // bcrypt hashing cost (12 is recommended)
	SessionTokenLength  = 32        // bytes for session token
	CSRFTokenLength     = 32        // bytes for CSRF token
	MaxLoginAttempts    = 5         // failed attempts before lockout
	LockoutMinutes      = 15        // lockout duration in minutes

	// File handling
	ThumbnailSize       = 300       // pixels (width/height for thumbnail)
	MaxFilenameLength   = 200       // characters
	MaxFilenameCounter  = 10000     // max attempts to find unique filename

	// Request limits
	MaxJSONBodyBytes    = 64 * 1024 // 64KB for JSON request bodies
	SmallJSONBodyBytes  = 1024      // 1KB for simple JSON (role updates, thresholds)

	// Session cleanup
	SessionCleanupHours = 1         // how often to clean expired sessions
)

