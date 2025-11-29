# Mnemosyne - Local Photo Cloud

A secure, self-hosted photo storage solution for your home network. Upload, browse, and manage your photos from any device on your WiFi.

## Features

- **Secure**: Password-protected with HTTPS encryption
- **Mobile-Friendly**: Works great on phones, tablets, and computers
- **Fast**: Automatic thumbnail generation for quick browsing
- **Simple**: Single executable, zero dependencies after building
- **Local Network Only**: Accessible only on your home WiFi
- **Easy Upload**: Drag-and-drop or click to browse
- **Full Control**: Upload, browse, download, and delete photos

<img width="1141" height="602" alt="Screenshot 2025-11-29 104149" src="https://github.com/user-attachments/assets/6f7e6494-939e-49a7-8af7-1cb984615c2a" />

<img width="1273" height="522" alt="Screenshot 2025-11-29 104200" src="https://github.com/user-attachments/assets/dc1090eb-4359-43ec-89cc-7083b8223ecd" />


## Security Features

- **HTTPS with self-signed TLS certificate**
- **bcrypt password hashing** (cost factor 12)
- **Brute force protection** (5 attempts → 15 min lockout)
- **CSRF protection** on all state-changing operations
- **Session management** with secure, HTTP-only cookies
- **File validation** (magic byte checking, not just extensions)
- **Security headers** (CSP, X-Frame-Options, etc.)

## Windows Setup

### Prerequisites

1. **Install Go** (if running from source or building yourself)
   - Download from: https://go.dev/download/
   - Install the Windows installer (.msi file)
   - Verify installation: Open PowerShell and run `go version`

### Installation

#### Option 1: Run from Source (Development)

1. **Clone or download this repository**
   ```powershell
   cd C:\Users\YourName\
   git clone <repository-url> Mnemosyne
   cd Mnemosyne
   ```

2. **Install dependencies**
   ```powershell
   go mod download
   ```

3. **Run the server**
   ```powershell
   go run .
   ```

#### Option 2: Build Executable (Production)

1. **Navigate to the project directory**
   ```powershell
   cd C:\Users\YourName\Mnemosyne
   ```

2. **Build the executable**
   ```powershell
   go build -o mnemosyne.exe .
   ```

3. **Run the executable**
   ```powershell
   .\mnemosyne.exe
   ```

### First Run

On first run, the application will:
1. Prompt you to create a password (or generate a random one)
2. Create a `config.json` file
3. Generate a self-signed SSL certificate
4. Create storage directories

Example first run:
```
No config found. Creating new configuration...
Enter password for photo cloud (or press Enter for random password): 
Configuration saved to config.json

Auto-generating self-signed certificate...
✓ Created: ./certs/server.crt
✓ Created: ./certs/server.key
⚠ Browser will show security warning - this is normal for self-signed certs
  Accept it once and you're set!

✓ Server is ready!
  Listen address: 0.0.0.0:8080
  Protocol: HTTPS (secure)

📱 Access from your devices at:
  https://192.168.1.100:8080
  https://localhost:8080

⚠  Note: You'll see a security warning for the self-signed certificate.
   This is normal - accept it to continue.

Press Ctrl+C to stop the server.
```

### Accessing the App

1. **Find your PC's IP address**
   - Open PowerShell: `ipconfig`
   - Look for "IPv4 Address" under your WiFi adapter
   - Example: `192.168.1.100`

2. **Access from any device on your WiFi**
   - On your phone/tablet/laptop browser, go to:
   - `https://192.168.1.100:8080` (replace with your actual IP)

3. **Accept the security certificate**
   - Your browser will show a security warning
   - This is normal for self-signed certificates
   - Click "Advanced" → "Proceed to site" (Chrome)
   - Or "Advanced" → "Accept the Risk" (Firefox)

4. **Login with your password**

## Configuration

The `config.json` file is created automatically on first run. You can edit it to customize settings:

```json
{
  "password": "your-password-here",
  "port": 8080,
  "storage_path": "./photos",
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
| `password` | (prompted) | Your login password |
| `port` | 8080 | Port to run the server on |
| `storage_path` | ./photos | Where photos are stored |
| `bind_address` | 0.0.0.0 | Network interface to bind to |
| `max_upload_mb` | 50 | Maximum file size per upload |
| `session_expiry_hours` | 24 | How long sessions last |
| `enable_https` | true | Use HTTPS (recommended) |
| `cert_path` | ./certs/server.crt | TLS certificate path |
| `key_path` | ./certs/server.key | TLS key path |

## Running as a Windows Service (Optional)

To run Mnemosyne automatically when Windows starts:

### Using Task Scheduler

1. Open Task Scheduler (`taskschd.msc`)
2. Create Basic Task → Name it "Mnemosyne Photo Cloud"
3. Trigger: "When the computer starts"
4. Action: "Start a program"
5. Program: `C:\Users\YourName\Mnemosyne\mnemosyne.exe`
6. Start in: `C:\Users\YourName\Mnemosyne`
7. Finish and test by restarting your PC

### Using NSSM (Non-Sucking Service Manager)

1. Download NSSM from: https://nssm.cc/download
2. Open PowerShell as Administrator
3. Run:
   ```powershell
   nssm install Mnemosyne "C:\Users\YourName\Mnemosyne\mnemosyne.exe"
   nssm start Mnemosyne
   ```

## Storage Location

By default, photos are stored in:
```
Mnemosyne/
└── photos/
    ├── originals/    (full-resolution photos)
    └── thumbnails/   (300px thumbnails)
```

You can change the storage location in `config.json` to use a different drive:
```json
{
  "storage_path": "D:\\Photos\\Mnemosyne"
}
```

## Firewall

Windows Firewall typically allows local network connections automatically. If you have issues accessing from other devices:

1. Open "Windows Defender Firewall with Advanced Security"
2. Inbound Rules → New Rule
3. Port → TCP → Specific port: 8080
4. Allow the connection
5. Apply to Private networks only
6. Name it "Mnemosyne Photo Cloud"

## Supported Image Formats

- JPEG/JPG
- PNG
- GIF
- WebP

Files are validated by their actual content (magic bytes), not just the extension.

## Browser Compatibility

Works on all modern browsers:
- Chrome/Edge
- Firefox
- Safari (iOS/macOS)
- Samsung Internet (Android)

## Troubleshooting

### Can't access from other devices

1. Check your PC's IP address: `ipconfig` in PowerShell
2. Verify both devices are on the same WiFi network
3. Check Windows Firewall settings
4. Make sure the server is running

### Certificate warnings

This is normal for self-signed certificates. The connection is still encrypted. Accept the certificate in your browser.

### Upload fails

1. Check file size (default max: 50MB per file)
2. Verify the file is a supported image format
3. Check available disk space

### Forgot password

1. Stop the server
2. Edit `config.json`
3. Change the `password` field to a new password
4. Delete the `password_hash` field
5. Restart the server (it will hash the new password)

## Building from Source

```powershell
# Get dependencies
go mod download

# Build for Windows
go build -o mnemosyne.exe .

# Build with optimizations (smaller binary)
go build -ldflags="-s -w" -o mnemosyne.exe .
```

## Project Structure

```
Mnemosyne/
├── main.go              # Entry point
├── config.go            # Configuration management
├── auth.go              # Authentication & sessions
├── photos.go            # Photo management
├── handlers.go          # HTTP handlers
├── cert.go              # TLS certificate generation
├── utils.go             # Utility functions
├── templates/
│   ├── login.html       # Login page
│   └── gallery.html     # Gallery interface
├── static/
│   ├── css/
│   │   └── style.css    # Styling
│   └── js/
│       └── app.js       # Frontend logic
├── photos/              # Created at runtime
├── certs/               # Created at runtime
└── config.json          # Created at runtime
```

## License

MIT License - Feel free to use and modify as needed.

## Support

For issues or questions, please open an issue on the repository.

---

**Made with ❤️ for home network photo storage**

