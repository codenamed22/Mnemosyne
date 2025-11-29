package main

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"
)

// App holds the application state
type App struct {
	config      *Config
	sessionMgr  *SessionManager
	photoMgr    *PhotoManager
	templates   *template.Template
}

// HandleLogin shows the login page or processes login
func (app *App) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Show login page
		app.templates.ExecuteTemplate(w, "login.html", nil)
		return
	}
	
	if r.Method == http.MethodPost {
		// Process login
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		
		password := r.FormValue("password")
		
		if err := app.sessionMgr.Login(w, r, password); err != nil {
			// Show login page with error
			app.templates.ExecuteTemplate(w, "login.html", map[string]string{
				"Error": err.Error(),
			})
			return
		}
		
		// Redirect to gallery
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
	// Validate session
	session, err := app.sessionMgr.ValidateSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	
	// Render gallery with CSRF token
	app.templates.ExecuteTemplate(w, "gallery.html", map[string]string{
		"CSRFToken": session.CSRFToken,
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
	mux.HandleFunc("GET /logout", app.HandleLogout)
	
	// Protected routes
	mux.HandleFunc("GET /", app.HandleGallery)
	
	// API routes
	mux.HandleFunc("POST /api/photos/upload", app.HandleUpload)
	mux.HandleFunc("GET /api/photos/list", app.HandleListPhotos)
	mux.HandleFunc("GET /api/photos/original/{filename}", app.HandleGetOriginal)
	mux.HandleFunc("GET /api/photos/thumbnail/{filename}", app.HandleGetThumbnail)
	mux.HandleFunc("DELETE /api/photos/{filename}", app.HandleDeletePhoto)
	
	// Static files - try embedded first, fallback to local
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err == nil {
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSubFS))))
	} else {
		// Fallback to local files for development
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	}
	
	// Apply middleware
	handler := securityHeadersMiddleware(mux)
	handler = loggingMiddleware(handler)
	
	return handler
}

