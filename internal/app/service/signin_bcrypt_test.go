package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// TestBcryptPasswordVerification_CorrectPassword tests the core logic of bcrypt password verification
// This simulates the behavior of password comparison in the SignIn method
func TestBcryptPasswordVerification_CorrectPassword(t *testing.T) {
	// Generate bcrypt hash for "admin123"
	plainPassword := "admin123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	fmt.Printf("Hashed password: %s\n", hashedPassword)
	assert.NoError(t, err)

	// Verify correct password
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(plainPassword))
	assert.NoError(t, err, "correct password should verify successfully")
}

// TestBcryptPasswordVerification_IncorrectPassword tests that incorrect password is correctly rejected
func TestBcryptPasswordVerification_IncorrectPassword(t *testing.T) {
	// Generate bcrypt hash for "admin123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)

	// Attempt to verify with wrong password
	err := bcrypt.CompareHashAndPassword(hashedPassword, []byte("wrongpassword"))
	assert.Error(t, err, "incorrect password should fail verification")
}

// TestBcryptPasswordVerification_EmptyPassword tests empty password
func TestBcryptPasswordVerification_EmptyPassword(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)

	// Attempt to verify with empty password
	err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(""))
	assert.Error(t, err, "empty password should fail verification")
}

// TestBcryptPasswordVerification_MultipleHashesSamePlaintext tests that identical passwords produce different hashes
func TestBcryptPasswordVerification_MultipleHashesSamePlaintext(t *testing.T) {
	plainPassword := "admin123"

	// Generate two different hashes for the same password
	hash1, _ := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	hash2, _ := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)

	// The two hashes should be different (because bcrypt uses random salt)
	assert.NotEqual(t, hash1, hash2, "the two hashes should be different (bcrypt uses random salt)")

	// But both should be able to verify the correct password
	assert.NoError(t, bcrypt.CompareHashAndPassword(hash1, []byte(plainPassword)))
	assert.NoError(t, bcrypt.CompareHashAndPassword(hash2, []byte(plainPassword)))
}

// TestBcryptPasswordVerification_CaseSensitive tests password case sensitivity
func TestBcryptPasswordVerification_CaseSensitive(t *testing.T) {
	plainPassword := "admin123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)

	// Password with different case should fail verification
	err := bcrypt.CompareHashAndPassword(hashedPassword, []byte("Admin123"))
	assert.Error(t, err, "password should be case-sensitive")
}

// TestBcryptPasswordVerification_LongPassword tests long password (bcrypt limits to 72 bytes max)
func TestBcryptPasswordVerification_LongPassword(t *testing.T) {
	// bcrypt supports passwords up to 72 bytes max, excess is ignored
	longPassword := "this_is_a_very_long_password_with_special_chars!@#$%"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(longPassword), bcrypt.DefaultCost)
	assert.NoError(t, err)

	// Verifying long password should succeed
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(longPassword))
	assert.NoError(t, err, "long password should verify successfully")

	// Incorrect long password should fail
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(longPassword+"x"))
	assert.Error(t, err, "incomplete long password should fail verification")
}

// TestBcryptPasswordVerification_SpecialCharacters tests password with special characters
func TestBcryptPasswordVerification_SpecialCharacters(t *testing.T) {
	specialPassword := "P@ssw0rd!#$%"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(specialPassword), bcrypt.DefaultCost)

	// Verifying password with special characters should succeed
	err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(specialPassword))
	assert.NoError(t, err, "password with special characters should verify successfully")
}

// TestBcryptPasswordVerification_DefaultCostParameter tests using default cost parameter
func TestBcryptPasswordVerification_DefaultCostParameter(t *testing.T) {
	password := "admin123"

	// Generate hash using DefaultCost (usually 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	assert.NoError(t, err)

	// Verification should succeed
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	assert.NoError(t, err, "hash generated with DefaultCost should verify successfully")
}

// TestSignInPasswordValidationLogic tests the core logic of password verification in SignIn
// This simulates how the actual SignIn method verifies passwords
func TestSignInPasswordValidationLogic(t *testing.T) {
	testCases := []struct {
		name            string
		plainPassword   string
		attemptPassword string
		shouldSucceed   bool
	}{
		{
			name:            "Correct password",
			plainPassword:   "admin123",
			attemptPassword: "admin123",
			shouldSucceed:   true,
		},
		{
			name:            "Wrong password",
			plainPassword:   "admin123",
			attemptPassword: "wrongpassword",
			shouldSucceed:   false,
		},
		{
			name:            "Empty password",
			plainPassword:   "admin123",
			attemptPassword: "",
			shouldSucceed:   false,
		},
		{
			name:            "Case mismatch",
			plainPassword:   "admin123",
			attemptPassword: "Admin123",
			shouldSucceed:   false,
		},
		{
			name:            "Long password verification",
			plainPassword:   "VerySecurePassword!@#$%^&*()",
			attemptPassword: "VerySecurePassword!@#$%^&*()",
			shouldSucceed:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Generate hash for correct password (simulate password stored in database)
			storedHash, err := bcrypt.GenerateFromPassword([]byte(tc.plainPassword), bcrypt.DefaultCost)
			assert.NoError(t, err)

			// Step 2: Verify with attempted password (simulate password comparison in SignIn)
			err = bcrypt.CompareHashAndPassword(storedHash, []byte(tc.attemptPassword))

			// Step 3: Check result
			if tc.shouldSucceed {
				assert.NoError(t, err, "password verification should succeed")
			} else {
				assert.Error(t, err, "password verification should fail")
			}
		})
	}
}
