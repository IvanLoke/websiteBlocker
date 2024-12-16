package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func getPasswordFilePath() string {
	return passwordFilePath
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func createPassword(reader *bufio.Reader) error {
	fmt.Print("Create new password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

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

func changePassword(reader *bufio.Reader) error {
	// Verify current password first
	if !verifyPassword(reader) {
		return fmt.Errorf("current password verification failed")
	}

	return createPassword(reader)
}
