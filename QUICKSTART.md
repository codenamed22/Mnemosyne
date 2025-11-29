# Quick Start Guide

## For Windows Users

### Running the Server (3 Steps)

1. **Download or Build**
   - Either download the pre-built `mnemosyne.exe`
   - Or build it yourself: `go build -o mnemosyne.exe .`

2. **Run the Server**
   ```powershell
   .\mnemosyne.exe
   ```
   - First time: You'll be prompted to create a password
   - It will show you URLs to access the app

3. **Access from Your Phone/Tablet**
   - Open your phone's browser
   - Go to the URL shown (e.g., `https://192.168.1.100:8080`)
   - Accept the certificate warning (one-time)
   - Login with your password
   - Start uploading photos!

### Example First Run

```
No config found. Creating new configuration...
Enter password for photo cloud (or press Enter for random password): mySecurePass123
Configuration saved to config.json

Auto-generating self-signed certificate...
✓ Created: ./certs/server.crt
✓ Created: ./certs/server.key

✓ Server is ready!
  Protocol: HTTPS (secure)

📱 Access from your devices at:
  https://192.168.1.100:8080
  https://localhost:8080
```

### Accessing the App

From any device on your WiFi:
- Open browser
- Go to: `https://YOUR_PC_IP:8080`
- Accept certificate warning
- Login with password
- Upload photos by dragging and dropping!

### Stopping the Server

Press `Ctrl+C` in the PowerShell window

---

That's it! See [README.md](README.md) for advanced configuration and troubleshooting.

