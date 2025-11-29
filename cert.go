package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// generateSelfSignedCert creates a self-signed TLS certificate
func generateSelfSignedCert(certPath, keyPath string) error {
	fmt.Println("Auto-generating self-signed certificate...")
	
	// Ensure cert directory exists
	certDir := filepath.Dir(certPath)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %v", err)
	}
	
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}
	
	// Get local IP addresses
	ips, err := getLocalIPs()
	if err != nil {
		fmt.Printf("Warning: couldn't get local IPs: %v\n", err)
		ips = []net.IP{net.ParseIP("127.0.0.1")}
	}
	
	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %v", err)
	}
	
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Mnemosyne Local Photo Cloud"},
			CommonName:   "Mnemosyne",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           ips,
		DNSNames:              []string{"localhost"},
	}
	
	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}
	
	// Save certificate
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %v", err)
	}
	defer certFile.Close()
	
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write cert: %v", err)
	}
	
	// Save private key
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	defer keyFile.Close()
	
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}
	
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}
	
	fmt.Printf("✓ Created: %s\n", certPath)
	fmt.Printf("✓ Created: %s\n", keyPath)
	fmt.Println("⚠ Browser will show security warning - this is normal for self-signed certs")
	fmt.Println("  Accept it once and you're set!")
	
	return nil
}

// getLocalIPs returns all non-loopback IPv4 addresses
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP
	
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP)
			}
		}
	}
	
	// Always include localhost
	ips = append(ips, net.ParseIP("127.0.0.1"))
	
	return ips, nil
}

// ensureCertificates checks if certificates exist and generates them if needed
func ensureCertificates(certPath, keyPath string) error {
	certExists := fileExists(certPath)
	keyExists := fileExists(keyPath)
	
	if certExists && keyExists {
		return nil
	}
	
	if certExists != keyExists {
		return fmt.Errorf("incomplete certificate pair (only one of cert/key exists)")
	}
	
	return generateSelfSignedCert(certPath, keyPath)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

