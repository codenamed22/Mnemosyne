package main

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"time"
)

// App holds the application state
type App struct {
	config     *Config
	db         *Database
	sessionMgr *SessionManager
	photoMgr   *PhotoManager
	templates  *template.Template
}

// HandleLogin shows the login page or processes login
func (app *App) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to gallery
	if _, err := app.sessionMgr.ValidateSession(r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		app.templates.ExecuteTemplate(w, "login.html", nil)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if err := app.sessionMgr.Login(w, r, username, password); err != nil {
			app.templates.ExecuteTemplate(w, "login.html", map[string]string{
				"Error": err.Error(),
			})
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// HandleRegister shows the registration page or processes registration
func (app *App) HandleRegister(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to gallery
	if _, err := app.sessionMgr.ValidateSession(r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		app.templates.ExecuteTemplate(w, "register.html", nil)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if password != confirmPassword {
			app.templates.ExecuteTemplate(w, "register.html", map[string]string{
				"Error": "Passwords do not match",
			})
			return
		}

		user, err := app.sessionMgr.Register(username, password)
		if err != nil {
			app.templates.ExecuteTemplate(w, "register.html", map[string]string{
				"Error": err.Error(),
			})
			return
		}

		// Auto-login after registration
		app.sessionMgr.Login(w, r, username, password)

		// Show success message based on role
		if user.Role == "admin" {
			log.Printf("First user '%s' registered as admin", username)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// HandleLogout logs out the user
func (app *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	app.sessionMgr.Logout(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HandleGallery shows the gallery page
func (app *App) HandleGallery(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.templates.ExecuteTemplate(w, "gallery.html", map[string]interface{}{
		"CSRFToken": session.CSRFToken,
		"Username":  session.Username,
		"IsAdmin":   session.IsAdmin(),
		"UserID":    session.UserID,
	})
}

// HandleAdmin shows the admin panel
func (app *App) HandleAdmin(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	app.templates.ExecuteTemplate(w, "admin.html", map[string]interface{}{
		"CSRFToken": session.CSRFToken,
		"Username":  session.Username,
	})
}

// HandleAPIGetUsers returns all users (admin only)
func (app *App) HandleAPIGetUsers(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	users, err := app.db.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}

	// Add photo count for each user
	type UserWithStats struct {
		*User
		PhotoCount int `json:"photo_count"`
	}

	usersWithStats := make([]UserWithStats, len(users))
	for i, user := range users {
		count, _ := app.db.GetUserPhotoCount(user.ID)
		usersWithStats[i] = UserWithStats{User: user, PhotoCount: count}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usersWithStats)
}

// HandleAPIDeleteUser deletes a user (admin only)
func (app *App) HandleAPIDeleteUser(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	userIDStr := r.PathValue("userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Can't delete yourself
	if userID == session.UserID {
		http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
		return
	}

	if err := app.db.DeleteUser(userID); err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "User deleted",
	})
}

// HandleAPIUpdateUserRole updates a user's role (admin only)
func (app *App) HandleAPIUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := app.sessionMgr.ValidateCSRF(r, session); err != nil {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	userIDStr := r.PathValue("userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Role != "admin" && body.Role != "user" {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	// Can't change your own role
	if userID == session.UserID {
		http.Error(w, "Cannot change your own role", http.StatusBadRequest)
		return
	}

	if err := app.db.UpdateUserRole(userID, body.Role); err != nil {
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Role updated",
	})
}

// HandleAPIGetStats returns system stats (admin only)
func (app *App) HandleAPIGetStats(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !session.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	users, _ := app.db.GetAllUsers()
	totalPhotos, _ := app.db.GetTotalPhotoCount()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_users":  len(users),
		"total_photos": totalPhotos,
	})
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// SetupRoutes configures all HTTP routes
func (app *App) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /login", app.HandleLogin)
	mux.HandleFunc("POST /login", app.HandleLogin)
	mux.HandleFunc("GET /register", app.HandleRegister)
	mux.HandleFunc("POST /register", app.HandleRegister)
	mux.HandleFunc("GET /logout", app.HandleLogout)

	// Protected routes
	mux.HandleFunc("GET /", app.HandleGallery)
	mux.HandleFunc("GET /admin", app.HandleAdmin)

	// Photo API routes
	mux.HandleFunc("POST /api/photos/upload", app.HandleUpload)
	mux.HandleFunc("GET /api/photos/my", app.HandleListMyPhotos)
	mux.HandleFunc("GET /api/photos/shared", app.HandleListSharedPhotos)
	mux.HandleFunc("GET /api/photos/all", app.HandleListAllPhotos)
	mux.HandleFunc("GET /api/photos/original/{userID}/{filename}", app.HandleGetOriginal)
	mux.HandleFunc("GET /api/photos/thumbnail/{userID}/{filename}", app.HandleGetThumbnail)
	mux.HandleFunc("DELETE /api/photos/{photoID}", app.HandleDeletePhoto)
	mux.HandleFunc("POST /api/photos/{photoID}/share", app.HandleSharePhoto)

	// Bulk operations
	mux.HandleFunc("POST /api/photos/bulk/share", app.HandleBulkShare)
	mux.HandleFunc("POST /api/photos/bulk/download", app.HandleBulkDownload)
	mux.HandleFunc("POST /api/photos/bulk/delete", app.HandleBulkDelete)

	// Admin API routes
	mux.HandleFunc("GET /api/admin/users", app.HandleAPIGetUsers)
	mux.HandleFunc("DELETE /api/admin/users/{userID}", app.HandleAPIDeleteUser)
	mux.HandleFunc("PUT /api/admin/users/{userID}/role", app.HandleAPIUpdateUserRole)
	mux.HandleFunc("GET /api/admin/stats", app.HandleAPIGetStats)

	// Static files
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err == nil {
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSubFS))))
	} else {
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	}

	// Apply middleware
	handler := securityHeadersMiddleware(mux)
	handler = loggingMiddleware(handler)

	return handler
}
