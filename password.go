package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func getPasswordFilePath() string {
	return passwordFilePath
}

// Function to hash password
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// Function to verify if password matches stored value
func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePassword checks if the password meets the security requirements
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	// Check for at least one uppercase letter
	if matched, _ := regexp.MatchString("[A-Z]", password); !matched {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}

	// Check for at least one lowercase letter
	if matched, _ := regexp.MatchString("[a-z]", password); !matched {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}

	// Check for at least one digit
	if matched, _ := regexp.MatchString("[0-9]", password); !matched {
		return fmt.Errorf("password must contain at least one digit")
	}

	// Check for at least one special character
	if matched, _ := regexp.MatchString("[!@#$%^&*(),.?\":{}|<>]", password); !matched {
		return fmt.Errorf("password must contain at least one special character")
	}

	return nil
}

// Function to create password
func createPassword(reader *bufio.Reader) error {
	fmt.Print("Create new password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	// Validate the new password
	if err := ValidatePassword(password); err != nil {
		return fmt.Errorf("password validation failed: %v", err)
	}

	fmt.Print("Confirm password: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)

	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("error hashing password: %v", err)
	}

	// Store the hashed password in the password file
	err = os.WriteFile(getPasswordFilePath(), []byte(hashedPassword), 0600)
	if err != nil {
		return fmt.Errorf("error saving password: %v", err)
	}

	return nil
}

// Function to get password from user and verify it
func verifyPassword(reader *bufio.Reader) bool {
	// Read stored password hash
	hashedBytes, err := os.ReadFile(getPasswordFilePath())
	if err != nil {
		fmt.Println("No password set. Please create a password first.")
		if err := createPassword(reader); err != nil {
			fmt.Printf("Error creating password: %v\n", err)
			return false
		}
		return true
	}

	// Get password from user
	fmt.Print("Enter password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	// Verify password
	if !checkPassword(password, string(hashedBytes)) {
		fmt.Println("Incorrect password")
		return false
	}

	return true
}

// Function to change password and rewrite to file
func changePassword(reader *bufio.Reader) error {
	// Verify current password first
	if !verifyPassword(reader) {
		return fmt.Errorf("current password verification failed")
	}

	return createPassword(reader)
}
