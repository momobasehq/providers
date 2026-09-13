package airtelcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

func PublicKey(raw string) (*rsa.PublicKey, error) {
	raw = strings.TrimSpace(raw)
	var der []byte
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		der = block.Bytes
	} else {
		b, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("airtel public key: %w", err)
		}
		der = b
	}
	if k, err := x509.ParsePKIXPublicKey(der); err == nil {
		if r, ok := k.(*rsa.PublicKey); ok {
			return r, nil
		}
	}
	if r, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return r, nil
	}
	return nil, fmt.Errorf("airtel public key: unsupported RSA key")
}

func EncryptPIN(pub *rsa.PublicKey, pin string) (string, error) {
	b, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(pin))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func Sign(pub *rsa.PublicKey, body []byte) (signature, keyHeader string, err error) {
	key := make([]byte, 32)
	iv := make([]byte, aes.BlockSize)
	if _, err = rand.Read(key); err != nil {
		return
	}
	if _, err = rand.Read(iv); err != nil {
		return
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	pad := aes.BlockSize - (len(body) % aes.BlockSize)
	padded := append(append([]byte(nil), body...), make([]byte, pad)...)
	for i := len(padded) - pad; i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)
	signature = base64.StdEncoding.EncodeToString(encrypted)
	keyMaterial := base64.StdEncoding.EncodeToString(key) + ":" + base64.StdEncoding.EncodeToString(iv)
	wrapped, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(keyMaterial))
	if err != nil {
		return "", "", err
	}
	keyHeader = base64.StdEncoding.EncodeToString(wrapped)
	return
}
