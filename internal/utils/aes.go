package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

func AESEncrypt(stringToEncrypt string) (encryptedString string, err error) {
	KEY := GetConfig("AES_KEY")
	key, _ := hex.DecodeString(KEY)
	plaintext := []byte(stringToEncrypt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return fmt.Sprintf("%x", ciphertext), nil
}

func AESDecrypt(encryptedString string) (decryptedString string, err error) {
	defer func() {
		if r := recover(); r != nil {
			decryptedString = ""
			err = errors.New("error in decrypting")
		}
	}()

	KEY := GetConfig("AES_KEY")
	key, _ := hex.DecodeString(KEY)
	enc, _ := hex.DecodeString(encryptedString)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()

	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", nil
	}

	return string(plaintext), nil
}
