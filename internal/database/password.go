package database

import "golang.org/x/crypto/bcrypt"

// HashPassword 密码哈希
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验密码
func CheckPassword(pw, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
