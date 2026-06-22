package security

import "encoding/base64"

const EncryptedPrefix = "!enc!"

func EncryptString(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	encrypted, err := encrypt([]byte(s))
	if err != nil {
		return "", err
	}
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(encrypted), nil
}

func DecryptString(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) < len(EncryptedPrefix) || s[:len(EncryptedPrefix)] != EncryptedPrefix {
		return s, nil
	}
	raw := s[len(EncryptedPrefix):]
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	plaintext, err := decrypt(data)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
