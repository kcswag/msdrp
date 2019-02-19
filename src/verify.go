package src

import (
	"crypto/sha1"
	"time"
)

// verify key
func getverifyval(verificationKey string) []byte {
	//b := []byte(verificationKey + "\f")
	b := sha1.Sum([]byte(time.Now().Format("2006-01-02 15") + verificationKey))
	return b[:]
}
