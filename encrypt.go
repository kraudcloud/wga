package main

import (
	"crypto/aes"
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"crypto/sha1"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

func Encrypt(r io.Reader, w io.Writer, password string) error {
	key := pbkdf2.Key([]byte(password), []byte("bobir"), 4096, 32, sha1.New)

	aes, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(cryptoRand.Reader, nonce); err != nil {
		return err
	}

	plaintext, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	if _, err := w.Write(nonce); err != nil {
		return err
	}

	if _, err := w.Write(ciphertext); err != nil {
		return err
	}

	return nil
}

func Decrypt(r io.Reader, w io.Writer, password string) error {
	key := pbkdf2.Key([]byte(password), []byte("bobir"), 4096, 32, sha1.New)

	aes, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(r, nonce); err != nil {
		return err
	}

	ciphertext, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	if _, err := w.Write(plaintext); err != nil {
		return err
	}

	return nil
}
