package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// Database wraps the SQLite connection
type Database struct {
	db *sql.DB
}

// User represents a user in the system
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // "admin" or "user"
	CreatedAt    time.Time `json:"created_at"`
}

// Photo represents photo metadata in the database
type Photo struct {
	ID           int64      `json:"id"`
	Filename     string     `json:"filename"`
	UserID       int64      `json:"user_id"`
	Username     string     `json:"username,omitempty"`
	IsShared     bool       `json:"is_shared"`
	IsArchived   bool       `json:"is_archived"`
	ArchivedAt   *time.Time `json:"archived_at,omitempty"`
	Size         int64      `json:"size"`
	UploadedAt   time.Time  `json:"uploaded_at"`
	ThumbnailURL string     `json:"thumbnail_url"`
	OriginalURL  string     `json:"original_url"`
}

// PhotoEmbedding represents a CLIP embedding for a photo
type PhotoEmbedding struct {
	PhotoID   int64     `json:"photo_id"`
	Embedding []byte    `json:"embedding"` // JSON-encoded float64 array
	CreatedAt time.Time `json:"created_at"`
}

// NewDatabase creates and initializes the database
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %v", err)
	}

	database := &Database{db: db}

	// Create tables
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return database, nil
}

// createTables creates the necessary database tables
func (d *Database) createTables() error {
	// Users table
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %v", err)
	}

	// Photos table
	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS photos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			is_shared BOOLEAN DEFAULT FALSE,
			size INTEGER NOT NULL,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create photos table: %v", err)
	}

	// Create index for faster queries
	_, err = d.db.Exec(`CREATE INDEX IF NOT EXISTS idx_photos_user_id ON photos(user_id)`)
	if err != nil {
		return fmt.Errorf("failed to create index: %v", err)
	}

	_, err = d.db.Exec(`CREATE INDEX IF NOT EXISTS idx_photos_shared ON photos(is_shared)`)
	if err != nil {
		return fmt.Errorf("failed to create shared index: %v", err)
	}

	// Add archive columns if they don't exist (migration)
	d.db.Exec(`ALTER TABLE photos ADD COLUMN is_archived BOOLEAN DEFAULT FALSE`)
	d.db.Exec(`ALTER TABLE photos ADD COLUMN archived_at DATETIME`)

	// Create archived photos index
	_, err = d.db.Exec(`CREATE INDEX IF NOT EXISTS idx_photos_archived ON photos(is_archived)`)
	if err != nil {
		return fmt.Errorf("failed to create archived index: %v", err)
	}

	// Photo embeddings table for CLIP vectors
	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS photo_embeddings (
			photo_id INTEGER PRIMARY KEY,
			embedding BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (photo_id) REFERENCES photos(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create photo_embeddings table: %v", err)
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// User methods

// CreateUser creates a new user
func (d *Database) CreateUser(username, password string) (*User, error) {
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}

	// Check if this is the first user (make them admin)
	var count int
	err = d.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %v", err)
	}

	role := "user"
	if count == 0 {
		role = "admin"
	}

	// Insert user
	result, err := d.db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)",
		username, string(hash), role,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	id, _ := result.LastInsertId()

	return &User{
		ID:       id,
		Username: username,
		Role:     role,
	}, nil
}

// GetUserByUsername retrieves a user by username
func (d *Database) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := d.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (d *Database) GetUserByID(id int64) (*User, error) {
	user := &User{}
	err := d.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	return user, nil
}

// GetAllUsers retrieves all users (for admin)
func (d *Database) GetAllUsers() ([]*User, error) {
	rows, err := d.db.Query(
		"SELECT id, username, role, created_at FROM users ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %v", err)
	}
	defer rows.Close()

	users := make([]*User, 0)
	for rows.Next() {
		user := &User{}
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// DeleteUser deletes a user by ID
func (d *Database) DeleteUser(id int64) error {
	_, err := d.db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// UpdateUserRole updates a user's role
func (d *Database) UpdateUserRole(id int64, role string) error {
	_, err := d.db.Exec("UPDATE users SET role = ? WHERE id = ?", role, id)
	return err
}

// VerifyPassword checks if the password matches the user's hash
func (u *User) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// IsAdmin returns true if user is an admin
func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

// Photo methods

// CreatePhoto adds a photo record to the database
func (d *Database) CreatePhoto(filename string, userID int64, size int64) (*Photo, error) {
	result, err := d.db.Exec(
		"INSERT INTO photos (filename, user_id, size) VALUES (?, ?, ?)",
		filename, userID, size,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create photo record: %v", err)
	}

	id, _ := result.LastInsertId()

	return &Photo{
		ID:       id,
		Filename: filename,
		UserID:   userID,
		Size:     size,
	}, nil
}

// GetPhotosByUser retrieves all photos for a user
func (d *Database) GetPhotosByUser(userID int64) ([]*Photo, error) {
	rows, err := d.db.Query(
		"SELECT id, filename, user_id, is_shared, size, uploaded_at FROM photos WHERE user_id = ? AND (is_archived = FALSE OR is_archived IS NULL) ORDER BY uploaded_at DESC",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos: %v", err)
	}
	defer rows.Close()

	return d.scanPhotos(rows)
}

// GetSharedPhotos retrieves all shared photos (family area)
func (d *Database) GetSharedPhotos() ([]*Photo, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.filename, p.user_id, p.is_shared, p.size, p.uploaded_at, u.username
		FROM photos p
		JOIN users u ON p.user_id = u.id
		WHERE p.is_shared = TRUE AND (p.is_archived = FALSE OR p.is_archived IS NULL)
		ORDER BY p.uploaded_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared photos: %v", err)
	}
	defer rows.Close()

	photos := make([]*Photo, 0)
	for rows.Next() {
		photo := &Photo{}
		if err := rows.Scan(&photo.ID, &photo.Filename, &photo.UserID, &photo.IsShared, &photo.Size, &photo.UploadedAt, &photo.Username); err != nil {
			return nil, fmt.Errorf("failed to scan photo: %v", err)
		}
		photos = append(photos, photo)
	}

	return photos, nil
}

// GetAllPhotos retrieves all photos (for admin)
func (d *Database) GetAllPhotos() ([]*Photo, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.filename, p.user_id, p.is_shared, p.size, p.uploaded_at, u.username
		FROM photos p
		JOIN users u ON p.user_id = u.id
		WHERE (p.is_archived = FALSE OR p.is_archived IS NULL)
		ORDER BY p.uploaded_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all photos: %v", err)
	}
	defer rows.Close()

	photos := make([]*Photo, 0)
	for rows.Next() {
		photo := &Photo{}
		if err := rows.Scan(&photo.ID, &photo.Filename, &photo.UserID, &photo.IsShared, &photo.Size, &photo.UploadedAt, &photo.Username); err != nil {
			return nil, fmt.Errorf("failed to scan photo: %v", err)
		}
		photos = append(photos, photo)
	}

	return photos, nil
}

// GetPhotoByID retrieves a photo by ID
func (d *Database) GetPhotoByID(id int64) (*Photo, error) {
	photo := &Photo{}
	err := d.db.QueryRow(
		"SELECT id, filename, user_id, is_shared, size, uploaded_at FROM photos WHERE id = ?",
		id,
	).Scan(&photo.ID, &photo.Filename, &photo.UserID, &photo.IsShared, &photo.Size, &photo.UploadedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get photo: %v", err)
	}

	return photo, nil
}

// GetPhotoByFilename retrieves a photo by filename and user ID
func (d *Database) GetPhotoByFilename(filename string, userID int64) (*Photo, error) {
	photo := &Photo{}
	err := d.db.QueryRow(
		"SELECT id, filename, user_id, is_shared, COALESCE(is_archived, FALSE), size, uploaded_at FROM photos WHERE filename = ? AND user_id = ?",
		filename, userID,
	).Scan(&photo.ID, &photo.Filename, &photo.UserID, &photo.IsShared, &photo.IsArchived, &photo.Size, &photo.UploadedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get photo: %v", err)
	}

	return photo, nil
}

// SetPhotoShared sets the shared status of a photo
func (d *Database) SetPhotoShared(id int64, shared bool) error {
	_, err := d.db.Exec("UPDATE photos SET is_shared = ? WHERE id = ?", shared, id)
	return err
}

// DeletePhoto deletes a photo record
func (d *Database) DeletePhoto(id int64) error {
	_, err := d.db.Exec("DELETE FROM photos WHERE id = ?", id)
	return err
}

// Helper function to scan photo rows
func (d *Database) scanPhotos(rows *sql.Rows) ([]*Photo, error) {
	photos := make([]*Photo, 0)
	for rows.Next() {
		photo := &Photo{}
		if err := rows.Scan(&photo.ID, &photo.Filename, &photo.UserID, &photo.IsShared, &photo.Size, &photo.UploadedAt); err != nil {
			return nil, fmt.Errorf("failed to scan photo: %v", err)
		}
		photos = append(photos, photo)
	}
	return photos, nil
}

// GetUserPhotoCount returns the number of photos for a user
func (d *Database) GetUserPhotoCount(userID int64) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM photos WHERE user_id = ?", userID).Scan(&count)
	return count, err
}

// GetTotalPhotoCount returns the total number of photos
func (d *Database) GetTotalPhotoCount() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM photos").Scan(&count)
	return count, err
}

// Archive methods

// ArchivePhoto marks a photo as archived
func (d *Database) ArchivePhoto(id int64) error {
	_, err := d.db.Exec(
		"UPDATE photos SET is_archived = TRUE, archived_at = CURRENT_TIMESTAMP WHERE id = ?",
		id,
	)
	return err
}

// UnarchivePhoto restores a photo from archive
func (d *Database) UnarchivePhoto(id int64) error {
	_, err := d.db.Exec(
		"UPDATE photos SET is_archived = FALSE, archived_at = NULL WHERE id = ?",
		id,
	)
	return err
}

// GetArchivedPhotos returns all archived photos for a user
func (d *Database) GetArchivedPhotos(userID int64) ([]*Photo, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.filename, p.user_id, u.username, p.is_shared, p.is_archived, p.archived_at, p.size, p.uploaded_at
		FROM photos p
		JOIN users u ON p.user_id = u.id
		WHERE p.user_id = ? AND p.is_archived = TRUE
		ORDER BY p.archived_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query archived photos: %v", err)
	}
	defer rows.Close()

	return d.scanPhotosWithArchive(rows)
}

// GetNonArchivedPhotos returns all non-archived photos for a user
func (d *Database) GetNonArchivedPhotos(userID int64) ([]*Photo, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.filename, p.user_id, u.username, p.is_shared, COALESCE(p.is_archived, FALSE), p.archived_at, p.size, p.uploaded_at
		FROM photos p
		JOIN users u ON p.user_id = u.id
		WHERE p.user_id = ? AND (p.is_archived = FALSE OR p.is_archived IS NULL)
		ORDER BY p.uploaded_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query photos: %v", err)
	}
	defer rows.Close()

	return d.scanPhotosWithArchive(rows)
}

// Helper function to scan photos with archive fields
func (d *Database) scanPhotosWithArchive(rows *sql.Rows) ([]*Photo, error) {
	photos := make([]*Photo, 0)
	for rows.Next() {
		photo := &Photo{}
		var archivedAt sql.NullTime
		if err := rows.Scan(
			&photo.ID, &photo.Filename, &photo.UserID, &photo.Username,
			&photo.IsShared, &photo.IsArchived, &archivedAt, &photo.Size, &photo.UploadedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan photo: %v", err)
		}
		if archivedAt.Valid {
			photo.ArchivedAt = &archivedAt.Time
		}
		photos = append(photos, photo)
	}
	return photos, nil
}

// Embedding methods

// SaveEmbedding saves a CLIP embedding for a photo
func (d *Database) SaveEmbedding(photoID int64, embedding []byte) error {
	_, err := d.db.Exec(`
		INSERT INTO photo_embeddings (photo_id, embedding) VALUES (?, ?)
		ON CONFLICT(photo_id) DO UPDATE SET embedding = ?, created_at = CURRENT_TIMESTAMP
	`, photoID, embedding, embedding)
	return err
}

// GetEmbedding retrieves the embedding for a photo
func (d *Database) GetEmbedding(photoID int64) ([]byte, error) {
	var embedding []byte
	err := d.db.QueryRow(
		"SELECT embedding FROM photo_embeddings WHERE photo_id = ?",
		photoID,
	).Scan(&embedding)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %v", err)
	}

	return embedding, nil
}

// GetAllEmbeddings retrieves embeddings for all non-archived photos of a user
func (d *Database) GetAllEmbeddings(userID int64) (map[int64][]byte, error) {
	rows, err := d.db.Query(`
		SELECT pe.photo_id, pe.embedding
		FROM photo_embeddings pe
		JOIN photos p ON pe.photo_id = p.id
		WHERE p.user_id = ? AND (p.is_archived = FALSE OR p.is_archived IS NULL)
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %v", err)
	}
	defer rows.Close()

	embeddings := make(map[int64][]byte)
	for rows.Next() {
		var photoID int64
		var embedding []byte
		if err := rows.Scan(&photoID, &embedding); err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %v", err)
		}
		embeddings[photoID] = embedding
	}

	return embeddings, nil
}

// GetPhotosWithoutEmbeddings returns photos that don't have embeddings yet
func (d *Database) GetPhotosWithoutEmbeddings(userID int64) ([]*Photo, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.filename, p.user_id, p.is_shared, p.size, p.uploaded_at
		FROM photos p
		LEFT JOIN photo_embeddings pe ON p.id = pe.photo_id
		WHERE p.user_id = ? AND pe.photo_id IS NULL AND (p.is_archived = FALSE OR p.is_archived IS NULL)
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query photos: %v", err)
	}
	defer rows.Close()

	return d.scanPhotos(rows)
}

// DeleteEmbedding deletes the embedding for a photo
func (d *Database) DeleteEmbedding(photoID int64) error {
	_, err := d.db.Exec("DELETE FROM photo_embeddings WHERE photo_id = ?", photoID)
	return err
}

// DeleteAllEmbeddings deletes all embeddings for a user
func (d *Database) DeleteAllEmbeddings(userID int64) (int64, error) {
	result, err := d.db.Exec(`
		DELETE FROM photo_embeddings 
		WHERE photo_id IN (SELECT id FROM photos WHERE user_id = ?)
	`, userID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetEmbeddingCount returns the number of embeddings for a user
func (d *Database) GetEmbeddingCount(userID int64) (int, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*)
		FROM photo_embeddings pe
		JOIN photos p ON pe.photo_id = p.id
		WHERE p.user_id = ?
	`, userID).Scan(&count)
	return count, err
}

