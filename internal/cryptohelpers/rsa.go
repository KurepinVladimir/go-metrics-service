package cryptohelpers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadPublicKeyFromFile загружает RSA public key из PEM-файла.
func LoadPublicKeyFromFile(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in public key file")
	}

	switch block.Type {
	case "PUBLIC KEY":
		pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKIX public key: %w", err)
		}
		pub, ok := pubAny.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not RSA")
		}
		return pub, nil
	case "RSA PUBLIC KEY":
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 public key: %w", err)
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("unsupported public key type: %s", block.Type)
	}
}

// LoadPrivateKeyFromFile загружает RSA private key из PEM-файла.
func LoadPrivateKeyFromFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key file")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 private key: %w", err)
		}
		return priv, nil
	case "PRIVATE KEY":
		keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		priv, ok := keyAny.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
		return priv, nil
	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}
}

func EncryptRSA(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	if pub == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	hash := sha256.New()
	return rsa.EncryptOAEP(hash, rand.Reader, pub, data, nil)
}

func DecryptRSA(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	if priv == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	hash := sha256.New()
	return rsa.DecryptOAEP(hash, rand.Reader, priv, data, nil)
}
