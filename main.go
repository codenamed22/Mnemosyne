package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path/filepath"
)

//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

const configPath = "config.json"

func main() {
	fmt.Println("🌟 Starting Mnemosyne Photo Cloud Server...")

	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Ensure necessary directories exist
	if err := config.EnsureDirectories(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Initialize database
	dbPath := filepath.Join(config.StoragePath, "mnemosyne.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Ensure TLS certificates exist if HTTPS is enabled
	if config.EnableHTTPS {
		if err := ensureCertificates(config.CertPath, config.KeyPath); err != nil {
			log.Fatalf("Failed to ensure certificates: %v", err)
		}
	}

	// Create app
	app, err := createApp(config, db)
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	// Setup routes
	handler := app.SetupRoutes()

	// Get local IP addresses
	ips := getLocalIPAddresses()

	// Start server
	addr := fmt.Sprintf("%s:%d", config.BindAddress, config.Port)

	fmt.Println("\n✓ Server is ready!")
	fmt.Printf("  Listen address: %s\n", addr)

	if config.EnableHTTPS {
		fmt.Println("  Protocol: HTTPS (secure)")
		fmt.Println("\n📱 Access from your devices at:")
		for _, ip := range ips {
			fmt.Printf("  https://%s:%d\n", ip, config.Port)
		}
		fmt.Println("\n⚠  Note: You'll see a security warning for the self-signed certificate.")
		fmt.Println("   This is normal - accept it to continue.")
	} else {
		fmt.Println("  Protocol: HTTP (not encrypted)")
		fmt.Println("\n📱 Access from your devices at:")
		for _, ip := range ips {
			fmt.Printf("  http://%s:%d\n", ip, config.Port)
		}
	}

	// Check if any users exist
	users, _ := db.GetAllUsers()
	if len(users) == 0 {
		fmt.Println("\n👤 No users found. The first user to register will become admin.")
	} else {
		fmt.Printf("\n👤 %d user(s) registered\n", len(users))
	}

	fmt.Println("\nPress Ctrl+C to stop the server.")

	// Start server
	if config.EnableHTTPS {
		if err := http.ListenAndServeTLS(addr, config.CertPath, config.KeyPath, handler); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else {
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}
}

// createApp creates an app instance
func createApp(config *Config, db *Database) (*App, error) {
	// Create session manager
	sessionMgr := NewSessionManager(db, config.SessionExpHrs)

	// Create photo manager
	photoMgr := NewPhotoManager(config.StoragePath, config.MaxUploadMB, db)

	// Parse embedded templates
	templatesSubFS, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to get templates subdirectory: %v", err)
	}

	templates, err := template.ParseFS(templatesSubFS, "*.html")
	if err != nil {
		// Fallback to local templates for development
		templates, err = template.ParseGlob("templates/*.html")
		if err != nil {
			return nil, fmt.Errorf("failed to parse templates: %v", err)
		}
	}

	app := &App{
		config:     config,
		db:         db,
		sessionMgr: sessionMgr,
		photoMgr:   photoMgr,
		templates:  templates,
	}

	return app, nil
}

// getLocalIPAddresses returns all local IP addresses
func getLocalIPAddresses() []string {
	var ips []string

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{"localhost"}
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}

	// Always include localhost
	ips = append(ips, "localhost")

	return ips
}
