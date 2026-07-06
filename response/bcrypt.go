package response

import "golang.org/x/crypto/bcrypt"

// bcryptCost controls password hashing cost.
// Lower cost improves login/register latency but reduces brute-force resistance.
const bcryptCost = 6 // default is 10

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(bytes), err
}
