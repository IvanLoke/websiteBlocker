package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var prefixes = [3]string{"www.", "https://", "http://"}
var suffixes = [2]string{".com", ".org"}

func FormatString(data string) string {
	return strings.ReplaceAll(strings.TrimSpace(strings.ToLower(data)), " ", "")
}

// Function to extract name from url
func GetNameFromURL(url string) string {
	for _, prefix := range prefixes {
		url = strings.TrimPrefix(url, prefix)
	}
	for _, suffix := range suffixes {
		url = strings.TrimSuffix(url, suffix)
	}
	return FormatString(url)
}

// Function to format time in "HH:MM" format
func FormatTime(time string) (string, error) {
	if len(time) < 4 || len(time) > 5 || (len(time) == 5 && time[2] != ':') {
		return "", errors.New("invalid time format")
	}

	if len(time) == 4 {
		time = time[:2] + ":" + time[2:]
	}

	// Split into hours and minutes
	parts := strings.Split(time, ":")
	if len(parts) != 2 {
		return "", errors.New("invalid time format")
	}

	// Parse and validate hours and minutes
	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 || hours > 23 {
		return "", errors.New("invalid hours")
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes > 59 {
		return "", errors.New("invalid minutes")
	}

	return time, nil
}

// Function to check if start time is before end time
func checkStartBeforeEnd(startTime string, endTime string) error {
	formattedStartTime, errStart := time.Parse("15:04", startTime)
	formatteedEndTime, errEnd := time.Parse("15:04", endTime)
	if errStart != nil {
		fmt.Println("Error parsing start time: ", errStart)
		return errStart
	}
	if errEnd != nil {
		fmt.Println("Error parsing end time: ", errEnd)
		return errEnd
	}
	if formatteedEndTime.Before(formattedStartTime) {
		return errors.New("end time cannot be before start time")
	}
	return nil
}

// Function to get duration from user input
func getDuration(reader *bufio.Reader) time.Duration {
	for {
		fmt.Print("Enter duration (e.g. 10s, 30m, 1h, 2h30m): ")
		input := readUserInput(reader)
		duration, err := time.ParseDuration(input)
		if err != nil {
			fmt.Println("Invalid duration format. Please try again.")
			continue
		}
		return duration
	}
}

// Function to check if the current time is within the specified time range
func isTimeInRange(currentTime, start, end string) bool {
	current, _ := time.Parse("15:04", currentTime)
	startTime, _ := time.Parse("15:04", start)
	endTime, _ := time.Parse("15:04", end)

	return current.After(startTime) && current.Before(endTime)
}

func checkForServiceFile() bool {
	_, err := os.Stat("/etc/systemd/system/selfcontrol.service")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Service file does not exist, creating...")
			return false
		}
	}
	return true
}

func createServiceFile() error {
	executable, _ := getDirectoryPaths()
	serviceContent := fmt.Sprintf(
		`[Unit]
	Description=Selfcontrol website blocker
	
	[Service]
	ExecStart="%s"
	Environment="SELFCONTROL_STARTUP=1"
	
	[Install]
	WantedBy=multi-user.target`, executable)

	// Write the service file
	err := os.WriteFile("/etc/systemd/system/selfcontrol.service", []byte(serviceContent), 0644)
	if err != nil {
		log.Printf("Error creating service file: %v", err)
		return fmt.Errorf("failed to create service file: %w", err)
	}
	log.Println("Service file created successfully.")

	// Reload systemd
	_, err = exec.Command("sudo", "systemctl", "daemon-reload").Output()
	if err != nil {
		log.Printf("Error reloading systemd: %v", err)
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	log.Println("Systemd reloaded successfully.")

	// Enable the service
	_, err = exec.Command("sudo", "systemctl", "enable", "selfcontrol.service").Output()
	if err != nil {
		log.Printf("Error enabling service: %v", err)
		return fmt.Errorf("failed to enable service: %w", err)
	}
	log.Println("Service enabled successfully.")

	// Start the service
	_, err = exec.Command("sudo", "systemctl", "start", "selfcontrol.service").Output()
	if err != nil {
		log.Printf("Error starting service: %v", err)
		return fmt.Errorf("failed to start service: %w", err)
	}
	log.Println("Service started successfully.")

	return nil
}

func initAbsPathToSelfControl() {
	constFilePath := "constants.go" // Update this to the actual path of constants.go

	// Read the current content of the constants file
	content, err := os.ReadFile(constFilePath)
	if err != nil {
		log.Printf("failed to read constants file: %v", err)
		return
	}

	_, newPath := getDirectoryPaths()
	// Update the path in the content
	updatedContent := strings.Replace(string(content), "placeholder", newPath, 1)

	// Write the updated content back to the file
	err = os.WriteFile(constFilePath, []byte(updatedContent), 0644)
	if err != nil {
		log.Printf("failed to write to constants file: %v", err)
		return
	}

	log.Println("Updated constants file successfully.")
}

func getDirectoryPaths() (executablePath string, executableDirectory string) {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable path: %v\n", err)
		return
	}

	// Optionally, get the directory of the executable
	exeDir := filepath.Dir(exePath)

	fmt.Printf("Executable Path: %s\n", exePath)
	fmt.Printf("Executable Directory: %s\n", exeDir)
	return exePath, exeDir
}
