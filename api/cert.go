package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//GenerateSelfSignedCert : gen auto de cert (repris de src/crypto/tls/generate_cert.go)
func GenerateSelfSignedCert() (string, string, error) {
	//chemin appli
	ex, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("exec path : %w", err)
	}
	cf := filepath.Dir(ex)
	cf = filepath.Join(cf, "cert")
	os.MkdirAll(cf, os.ModePerm)
	certPath := filepath.Join(cf, "cert.pem")
	keyPath := filepath.Join(cf, "key.pem")

	if _, err := os.Stat(certPath); err == nil {
		//fichier déja présent
		return certPath, keyPath, nil
	}

	// variable pour le cert
	//hostname
	host, err := os.Hostname()
	if err != nil {
		return "", "", fmt.Errorf("os.Hostname() %w", err)
	}
	//durée de validité
	var notBefore time.Time = time.Now()
	var notAfter = notBefore.Add(time.Hour * 24 * 3650) //10ans
	//own ca
	isCA := true

	//clé
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"CmdAgen SelfCert"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	//cert.pem
	certOut, err := os.Create(certPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open cert.pem for writing: %w", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to cert.pem: %w", err)
	}
	if err := certOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing cert.pem: %w", err)
	}

	//key.pem
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("unable to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to key.pem: %w", err)
	}
	if err := keyOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing key.pem: %w", err)
	}
	return certPath, keyPath, nil
}
