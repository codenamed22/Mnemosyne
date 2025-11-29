package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "mnemosyne_session"
	csrfTokenName     = "csrf_token"
	bcryptCost        = 12
	maxLoginAttempts  = 5
	lockoutDuration   = 15 * time.Minute
)

// Session represents a user session
type Session struct {
	Token     string
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
	passwordHash  string
	sessionExpiry time.Duration
	mu            sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(password string, sessionExpiryHours int) (*SessionManager, error) {
	var hash string
	var err error
	
	// If password is provided, hash it
	if password != "" {
		hash, err = hashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}
	}
	
	sm := &SessionManager{
		sessions:      make(map[string]*Session),
		loginAttempts: make(map[string]*LoginAttempt),
		passwordHash:  hash,
		sessionExpiry: time.Duration(sessionExpiryHours) * time.Hour,
	}
	
	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()
	
	return sm, nil
}

// SetPasswordHash sets a pre-hashed password
func (sm *SessionManager) SetPasswordHash(hash string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.passwordHash = hash
}

// hashPassword creates a bcrypt hash of the password
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// verifyPassword checks if the password matches the hash
func verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
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
func (sm *SessionManager) Login(w http.ResponseWriter, r *http.Request, password string) error {
	ip := getClientIP(r)
	
	// Check brute force protection
	if err := sm.checkBruteForce(ip); err != nil {
		return err
	}
	
	// Verify password
	sm.mu.RLock()
	hash := sm.passwordHash
	sm.mu.RUnlock()
	
	if !verifyPassword(password, hash) {
		sm.recordFailedAttempt(ip)
		return fmt.Errorf("invalid password")
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
		Secure:   r.TLS != nil, // Set Secure flag if HTTPS
		SameSite: http.SameSiteStrictMode,
	})
	
	return nil
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
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		return ip
	}
	
	// Check X-Real-IP header
	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

