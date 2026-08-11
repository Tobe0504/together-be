package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func HashPIN(pin string) (string, error) {
	return HashPassword(pin)
}

func VerifyPIN(hash, pin string) bool {
	return VerifyPassword(hash, pin)
}
