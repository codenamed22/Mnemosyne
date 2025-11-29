# 📸 Mnemosyne - Family Photo Cloud

A secure, self-hosted photo storage solution for your home network. Multiple family members can upload, browse, and share photos - all accessible from any device on your WiFi.

## Features

- **👨‍👩‍👧‍👦 Multi-User**: Each family member gets their own account and private photos
- **🔒 Secure**: HTTPS encryption, password protection, and role-based access
- **👑 Admin Control**: First user becomes admin, can manage all users and photos
- **🤝 Family Sharing**: Share photos to a family area visible to everyone
- **📱 Mobile-Friendly**: Works great on phones, tablets, and computers
- **🚀 Fast**: Automatic thumbnail generation for quick browsing
- **💾 Simple**: Single executable, zero dependencies after building

## Security Features

- **Self-registration** with first user becoming admin
- **HTTPS with self-signed TLS certificates**
- **bcrypt password hashing**
- **Brute force protection** (5 attempts → 15 min lockout)
- **CSRF protection** on all state-changing operations
- **Per-user photo storage** with access control
- **Session management** with secure, HTTP-only cookies

<img width="379" height="570" alt="image" src="https://github.com/user-attachments/assets/2bbc5cbe-a006-430d-a744-2d9bf3b49e47" />

<img width="1393" height="498" alt="image" src="https://github.com/user-attachments/assets/0f2ad050-14c1-4cbb-b07b-08720a099247" />

<img width="1510" height="770" alt="image" src="https://github.com/user-attachments/assets/3350068c-b323-439e-a69e-3df00a5756be" />

<img width="1510" height="770" alt="image" src="https://github.com/user-attachments/assets/5f1a19fa-01ae-47d1-9c35-f2402cb4a62e" />



## Quick Start

### Windows Setup

1. **Download or Build**
   ```powershell
   go build -o mnemosyne.exe .
   ```

2. **Run the Server**
   ```powershell
   .\mnemosyne.exe
   ```

3. **Register Your Account**
   - Open browser to `https://YOUR_PC_IP:8080`
   - Accept the certificate warning
   - Click "Register" to create your account
   - **First user automatically becomes admin!**

4. **Invite Family**
   - Share the URL with family members on your WiFi
   - They can register their own accounts

### Example First Run

```
🌟 Starting Mnemosyne Photo Cloud Server...
No config found. Creating default configuration...
Configuration saved to config.json

Auto-generating self-signed certificate...
✓ Created: ./certs/server.crt
✓ Created: ./certs/server.key

✓ Server is ready!
  Protocol: HTTPS (secure)

📱 Access from your devices at:
  https://192.168.1.100:8080
  https://localhost:8080

👤 No users found. The first user to register will become admin.

Press Ctrl+C to stop the server.
```

## User Roles

### Admin
- View and delete **all** photos
- Manage users (promote, demote, delete)
- Access admin panel at `/admin`

### User
- Upload, view, and delete **own** photos
- Share photos to family area
- View family area photos

## Photo Visibility

| Location | Who Can See |
|----------|-------------|
| **My Photos** | Only the owner |
| **Family Area** | All logged-in users |
| **All Photos** (Admin) | Admin only |

## Configuration

The `config.json` file is created automatically:

```json
{
  "port": 8080,
  "storage_path": "./data",
  "bind_address": "0.0.0.0",
  "max_upload_mb": 50,
  "session_expiry_hours": 24,
  "enable_https": true,
  "cert_path": "./certs/server.crt",
  "key_path": "./certs/server.key"
}
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `port` | 8080 | Port to run the server on |
| `storage_path` | ./data | Where data and photos are stored |
| `bind_address` | 0.0.0.0 | Network interface to bind to |
| `max_upload_mb` | 50 | Maximum file size per upload |
| `session_expiry_hours` | 24 | How long sessions last |
| `enable_https` | true | Use HTTPS (recommended) |

## Storage Structure

```
data/
├── mnemosyne.db          # SQLite database (users, photos metadata)
└── users/
    ├── 1/                # User ID folders
    │   ├── originals/    # Full-size photos
    │   └── thumbnails/   # 300px thumbnails
    ├── 2/
    └── ...
```

## API Endpoints

### Public
- `GET/POST /login` - Login page
- `GET/POST /register` - Registration page
- `GET /logout` - Logout

### Protected (User)
- `GET /` - Gallery page
- `POST /api/photos/upload` - Upload photo
- `GET /api/photos/my` - List own photos
- `GET /api/photos/shared` - List family area photos
- `GET /api/photos/original/{userID}/{filename}` - Get original
- `GET /api/photos/thumbnail/{userID}/{filename}` - Get thumbnail
- `DELETE /api/photos/{photoID}` - Delete photo
- `POST /api/photos/{photoID}/share` - Toggle family sharing

### Admin Only
- `GET /admin` - Admin panel
- `GET /api/photos/all` - List all photos
- `GET /api/admin/users` - List all users
- `DELETE /api/admin/users/{userID}` - Delete user
- `PUT /api/admin/users/{userID}/role` - Change user role
- `GET /api/admin/stats` - System stats

## Running as a Windows Service

### Using Task Scheduler

1. Open Task Scheduler (`taskschd.msc`)
2. Create Basic Task → "Mnemosyne Photo Cloud"
3. Trigger: "When the computer starts"
4. Action: Start `mnemosyne.exe`
5. Start in: The folder containing the exe

### Using NSSM

```powershell
nssm install Mnemosyne "C:\Path\To\mnemosyne.exe"
nssm start Mnemosyne
```

## Supported Image Formats

- JPEG/JPG
- PNG
- GIF
- WebP

Files are validated by content (magic bytes), not just extension.

## Browser Compatibility

- Chrome/Edge
- Firefox
- Safari (iOS/macOS)
- Samsung Internet

## Building from Source

```bash
# Get dependencies
go mod download

# Build
go build -o mnemosyne.exe .

# Build optimized
go build -ldflags="-s -w" -o mnemosyne.exe .
```

**Note**: Building requires CGO for SQLite. On Windows, you may need to install GCC (e.g., via MSYS2 or TDM-GCC).

## Troubleshooting

### Can't access from other devices
1. Check PC's IP: `ipconfig` in PowerShell
2. Verify same WiFi network
3. Check Windows Firewall (allow port 8080)

### Certificate warnings
Normal for self-signed certificates. Accept to continue.

### Registration not working
- Username: 3-32 characters, letters/numbers/underscores
- Password: minimum 6 characters

### SQLite build errors
Install GCC for CGO:
- Windows: Install TDM-GCC or MSYS2
- Mac: `xcode-select --install`
- Linux: `apt install gcc` or equivalent

## Project Structure

```
Mnemosyne/
├── main.go              # Entry point
├── config.go            # Configuration
├── database.go          # SQLite database
├── auth.go              # Authentication
├── photos.go            # Photo management
├── handlers.go          # HTTP handlers
├── cert.go              # TLS certificates
├── utils.go             # Utilities
├── templates/
│   ├── login.html
│   ├── register.html
│   ├── gallery.html
│   └── admin.html
├── static/
│   ├── css/style.css
│   └── js/
│       ├── app.js
│       └── admin.js
└── data/                # Created at runtime
```

## License

MIT License

---

**Made with ❤️ for family photo storage**
