package rooms

import (
	"crypto/rand"
	"math/big"
)

const joinCodeAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

func NewRoomID() (string, error) {
	return randomString(joinCodeAlphabet, 20)
}

func NewJoinCode() (string, error) {
	return randomString(joinCodeAlphabet, 6)
}

func randomString(alphabet string, n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b), nil
}
