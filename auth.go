package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "mnemosyne_session"
	csrfTokenName     = "csrf_token"
	maxLoginAttempts  = 5
	lockoutDuration   = 15 * time.Minute
)

// Session represents a user session
type Session struct {
	Token     string
	UserID    int64
	Username  string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
	CSRFToken string
}

// LoginAttempt tracks failed login attempts
type LoginAttempt struct {
	Count       int
	LockedUntil time.Time
}

// SessionManager handles session management and authentication
type SessionManager struct {
	sessions      map[string]*Session
	loginAttempts map[string]*LoginAttempt
	sessionExpiry time.Duration
	db            *Database
	mu            sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(db *Database, sessionExpiryHours int) *SessionManager {
	sm := &SessionManager{
		sessions:      make(map[string]*Session),
		loginAttempts: make(map[string]*LoginAttempt),
		sessionExpiry: time.Duration(sessionExpiryHours) * time.Hour,
		db:            db,
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// checkBruteForce checks if the IP is locked out due to too many attempts
func (sm *SessionManager) checkBruteForce(ip string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	attempt, exists := sm.loginAttempts[ip]
	if !exists {
		return nil
	}

	// Check if still locked out
	if time.Now().Before(attempt.LockedUntil) {
		remaining := time.Until(attempt.LockedUntil).Round(time.Second)
		return fmt.Errorf("too many failed attempts, try again in %v", remaining)
	}

	// Lockout expired, reset
	if time.Now().After(attempt.LockedUntil) {
		delete(sm.loginAttempts, ip)
	}

	return nil
}

// recordFailedAttempt records a failed login attempt
func (sm *SessionManager) recordFailedAttempt(ip string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	attempt, exists := sm.loginAttempts[ip]
	if !exists {
		attempt = &LoginAttempt{Count: 0}
		sm.loginAttempts[ip] = attempt
	}

	attempt.Count++

	// Lock out after max attempts
	if attempt.Count >= maxLoginAttempts {
		attempt.LockedUntil = time.Now().Add(lockoutDuration)
	}
}

// resetFailedAttempts resets failed login attempts for an IP
func (sm *SessionManager) resetFailedAttempts(ip string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.loginAttempts, ip)
}

// Login authenticates a user and creates a session
func (sm *SessionManager) Login(w http.ResponseWriter, r *http.Request, username, password string) error {
	ip := getClientIP(r)

	// Check brute force protection
	if err := sm.checkBruteForce(ip); err != nil {
		return err
	}

	// Get user from database
	user, err := sm.db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("authentication failed")
	}
	if user == nil {
		sm.recordFailedAttempt(ip)
		return fmt.Errorf("invalid username or password")
	}

	// Verify password
	if !user.VerifyPassword(password) {
		sm.recordFailedAttempt(ip)
		return fmt.Errorf("invalid username or password")
	}

	// Reset failed attempts on successful login
	sm.resetFailedAttempts(ip)

	// Create session
	token, err := generateRandomToken(32)
	if err != nil {
		return fmt.Errorf("failed to generate session token: %v", err)
	}

	csrfToken, err := generateRandomToken(32)
	if err != nil {
		return fmt.Errorf("failed to generate CSRF token: %v", err)
	}

	session := &Session{
		Token:     token,
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(sm.sessionExpiry),
		CSRFToken: csrfToken,
	}

	sm.mu.Lock()
	sm.sessions[token] = session
	sm.mu.Unlock()

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sm.sessionExpiry.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	return nil
}

// Register creates a new user account
func (sm *SessionManager) Register(username, password string) (*User, error) {
	// Validate input
	if len(username) < 3 || len(username) > 32 {
		return nil, fmt.Errorf("username must be between 3 and 32 characters")
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	// Check if username already exists
	existing, err := sm.db.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("registration failed")
	}
	if existing != nil {
		return nil, fmt.Errorf("username already taken")
	}

	// Create user
	user, err := sm.db.CreateUser(username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	return user, nil
}

// Logout destroys a session
func (sm *SessionManager) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}

	sm.mu.Lock()
	delete(sm.sessions, cookie.Value)
	sm.mu.Unlock()

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// ValidateSession checks if a session is valid
func (sm *SessionManager) ValidateSession(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}

	sm.mu.RLock()
	session, exists := sm.sessions[cookie.Value]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("invalid session")
	}

	if time.Now().After(session.ExpiresAt) {
		sm.mu.Lock()
		delete(sm.sessions, cookie.Value)
		sm.mu.Unlock()
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// ValidateCSRF checks if the CSRF token is valid
func (sm *SessionManager) ValidateCSRF(r *http.Request, session *Session) error {
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.FormValue("csrf_token")
	}

	if token == "" {
		return fmt.Errorf("missing CSRF token")
	}

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		return fmt.Errorf("invalid CSRF token")
	}

	return nil
}

// IsAdmin checks if the session user is an admin
func (s *Session) IsAdmin() bool {
	return s.Role == "admin"
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		sm.mu.Lock()
		for token, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, token)
			}
		}

		// Also cleanup old login attempts
		for ip, attempt := range sm.loginAttempts {
			if now.After(attempt.LockedUntil.Add(1 * time.Hour)) {
				delete(sm.loginAttempts, ip)
			}
		}
		sm.mu.Unlock()
	}
}

// getClientIP extracts the client IP from the request
// SECURITY: Only use RemoteAddr to prevent IP spoofing attacks on brute force protection.
// X-Forwarded-For and X-Real-IP headers are easily spoofable and should not be trusted
// for security-critical decisions like rate limiting.
// If behind a reverse proxy, configure the proxy to set RemoteAddr correctly.
func getClientIP(r *http.Request) string {
	// Extract IP from RemoteAddr (format: "IP:port" or just "IP")
	ip := r.RemoteAddr
	
	// Handle IPv6 addresses in brackets [::1]:port
	if len(ip) > 0 && ip[0] == '[' {
		if idx := strings.Index(ip, "]:"); idx != -1 {
			return ip[1:idx]
		}
		return strings.Trim(ip, "[]")
	}
	
	// Handle IPv4 addresses ip:port
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		return ip[:idx]
	}
	
	return ip
}
