// Package fwapp is a Firewalla App API client for local box control.
// Protocol behavior follows Firewalla's published encipher implementation
// and lab pairing captures.
package fwapp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Community Additional Pairing client identity (public protocol constant).
const (
	ProtocolClientID     = "com.rottiesoft.circle"
	ProtocolClientSecret = "fbb05afa-9145-41f1-8076-9de8be56f104"
	ProtocolClientVer    = "1.60.0"
	CloudAPIBase         = "https://firewalla.encipher.io/app/api/v2"
)

var (
	ErrBadQR        = errors.New("invalid pairing QR JSON")
	ErrCrypto       = errors.New("crypto failure")
	ErrNotPaired    = errors.New("firewalla control not paired")
	ErrLocalUnreach = errors.New("firewalla box unreachable on LAN")
)

// KeyPair holds PEM-encoded RSA keys for ETP-style membership.
type KeyPair struct {
	PrivatePEM string
	PublicPEM  string
	priv       *rsa.PrivateKey
}

// GenerateKeyPair creates a 2048-bit RSA keypair.
func GenerateKeyPair() (*KeyPair, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return keyPairFromPriv(priv)
}

func keyPairFromPriv(priv *rsa.PrivateKey) (*KeyPair, error) {
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return &KeyPair{PrivatePEM: string(privPEM), PublicPEM: string(pubPEM), priv: priv}, nil
}

// ParseKeyPair loads a PEM keypair.
func ParseKeyPair(privPEM, pubPEM string) (*KeyPair, error) {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		return nil, fmt.Errorf("%w: private pem", ErrCrypto)
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// try PKCS8
		k, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("%w: %v", ErrCrypto, err)
		}
		var ok bool
		priv, ok = k.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%w: not rsa", ErrCrypto)
		}
	}
	kp := &KeyPair{PrivatePEM: privPEM, PublicPEM: pubPEM, priv: priv}
	if strings.TrimSpace(pubPEM) == "" {
		pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		if err != nil {
			return nil, err
		}
		kp.PublicPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	}
	return kp, nil
}

func (k *KeyPair) private() *rsa.PrivateKey { return k.priv }

// RSADecryptBase64 decrypts a base64 RSA-OAEP or PKCS1v15 ciphertext (try PKCS1v15 first — Firewalla default).
func (k *KeyPair) RSADecryptBase64(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("%w: b64", ErrCrypto)
	}
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, k.priv, raw)
	if err != nil {
		plain, err = rsa.DecryptOAEP(sha1.New(), rand.Reader, k.priv, raw, nil)
		if err != nil {
			return "", fmt.Errorf("%w: rsa decrypt", ErrCrypto)
		}
	}
	return string(plain), nil
}

// aesKey32 takes the first 32 UTF-8 bytes of the symmetric key string (Firewalla encipher convention).
func aesKey32(sym string) []byte {
	b := []byte(sym)
	if len(b) >= 32 {
		return b[:32]
	}
	out := make([]byte, 32)
	copy(out, b)
	return out
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrCrypto
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(data) {
		return nil, ErrCrypto
	}
	for i := len(data) - pad; i < len(data); i++ {
		if data[i] != byte(pad) {
			return nil, ErrCrypto
		}
	}
	return data[:len(data)-pad], nil
}

// AESEncryptLegacy encrypts UTF-8 plaintext with AES-256-CBC and a zero IV; returns bare base64 ciphertext.
func AESEncryptLegacy(symKey, plaintext string) (string, error) {
	block, err := aes.NewCipher(aesKey32(symKey))
	if err != nil {
		return "", err
	}
	iv := make([]byte, aes.BlockSize) // zero IV
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

// AESDecryptMessage decrypts a Firewalla message value (legacy bare base64 or JSON envelope).
func AESDecryptMessage(symKey, ciphertext string) (string, error) {
	key := aesKey32(symKey)
	ct := ciphertext
	iv := make([]byte, aes.BlockSize)
	if len(ciphertext) > 0 && (ciphertext[0] == '{' || ciphertext[0] == '"') {
		var env struct {
			Alg     string `json:"alg"`
			IV      string `json:"iv"`
			Message string `json:"message"`
			Tag     string `json:"tag"`
			CT      string `json:"ct"`
		}
		if err := json.Unmarshal([]byte(ciphertext), &env); err != nil {
			// fall through as legacy
		} else if env.Message != "" || env.CT != "" {
			if env.Alg == "gcm" {
				return "", fmt.Errorf("%w: gcm responses not supported yet", ErrCrypto)
			}
			ct = env.Message
			if ct == "" {
				ct = env.CT
			}
			if env.IV != "" {
				raw, err := base64.StdEncoding.DecodeString(env.IV)
				if err != nil || len(raw) != aes.BlockSize {
					return "", fmt.Errorf("%w: bad iv", ErrCrypto)
				}
				iv = raw
			}
		}
	}
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		return "", fmt.Errorf("%w: b64 ct", ErrCrypto)
	}
	if len(raw)%aes.BlockSize != 0 {
		return "", fmt.Errorf("%w: ct length", ErrCrypto)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, raw)
	unpadded, err := pkcs7Unpad(plain)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

// QRBootstrapKey builds the AES key used to decrypt QR.ek (license[:8]+seed, or first-bind prefix).
func QRBootstrapKey(license, seed string, firstBind bool) string {
	prefix := "cybersecuritymadesimple"
	if !firstBind && len(license) >= 8 {
		prefix = license[:8]
	}
	return prefix + seed
}

// DecryptQRPayload decrypts the QR ek field to rendezvous material.
func DecryptQRPayload(license, seed, ek string, firstBind bool) (string, error) {
	return AESDecryptMessage(QRBootstrapKey(license, seed, firstBind), ek)
}

// RandomID returns a UUID-like string for message ids.
func RandomID() string {
	var b [16]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
