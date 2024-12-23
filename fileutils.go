package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Function to update the hosts file
func updateHostsFile(sites []string) error {
	hostsMu.Lock()         // Lock the mutex
	defer hostsMu.Unlock() // Ensure it gets unlocked at the end

	// Read the current contents of the hosts file
	content, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return err
	}

	// Open the hosts file for appending
	file, err := os.OpenFile("/etc/hosts", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close() // Ensure the file is closed once at the end

	// Check if the marker already exists
	if !strings.Contains(string(content), "# Added by selfcontrol") {
		// Add the marker
		if _, err := file.WriteString("\n# Added by selfcontrol\n"); err != nil {
			return err
		}
	}

	// Add new entries to the hosts file
	for _, site := range sites {
		if !strings.Contains(string(content), site) {
			if _, err := file.WriteString(fmt.Sprintf("127.0.0.1 %s\n", site)); err != nil {
				return err
			}
		}
	}

	return nil
}

// Functions for schedules.yaml

// Function to write to yaml file
func writeAndSave(filename string, data interface{}) error {
	// Write to original file
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	if err := encoder.Encode(&data); err != nil {
		return err
	}

	return nil
}
