package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Struct to hold the YAML data
type Config struct {
	Sites         []string               `yaml:"sites"`
	Schedules     map[string][]TimeRange `yaml:"schedules"`
	CurrentStatus CurrentStatus          `yaml:"current_status"`
}

type CurrentStatus struct {
	StartedAt string `yaml:"started_at"`
	Mode      string `yaml:"mode"`
}

type TimeRange struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

func readConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}

	return &config, nil
}

// Function that prints sites to block if it is time to block sites. Returns the end time of the block
func printSitesToBlock(config *Config, forStatus bool) (string, error) {
	currentDay := strings.ToLower(time.Now().Weekday().String()) // Get the current day of the week
	currentTime := time.Now().Format("15:04")                    // Get current time in HH:MM format

	// Check if there are schedules for the current day
	if schedules, exists := config.Schedules[currentDay]; exists {
		fmt.Printf("Checking schedules for %s...\n", currentDay) // Log the current day being checked
		for _, schedule := range schedules {
			// Check if the current time falls within any of the scheduled time ranges
			if isTimeInRange(currentTime, schedule.Start, schedule.End) {
				if forStatus {
					fmt.Println("*****Sites that are currently blocked*****")
				} else {
					fmt.Println("Sites to block:")
				}
				for i, site := range config.Sites {
					fmt.Printf("%s\n", fmt.Sprintf("%d: %s", i+1, site))
				}
				return schedule.End, nil // Exit after printing sites for the current time range
			}
		}
		return "", fmt.Errorf("no sites to block at this time") // Return an error if no sites were found
	} else {
		// Return an error if no schedules were found or no sites to block
		return "", fmt.Errorf("no schedules found for %s", currentDay)
	}
}

func isTimeInRange(currentTime, start, end string) bool {
	current, _ := time.Parse("15:04", currentTime)
	startTime, _ := time.Parse("15:04", start)
	endTime, _ := time.Parse("15:04", end)

	return current.After(startTime) && current.Before(endTime)
}

func blockSitesStrict(all bool, yamlFile string, specificSite string, isInBackground bool) error {
	var sites []string
	// Read sites from the specified YAML file
	headerSites, err := readConfig(yamlFile)
	expiryTime, _ := printSitesToBlock(headerSites, false)
	fmt.Println("Blocking sites until", expiryTime)
	currentDate := time.Now()
	expiryTimeParsed, _ := time.Parse("15:04", expiryTime)
	combinedTime := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(),
		expiryTimeParsed.Hour(), expiryTimeParsed.Minute(), 0, 0, currentDate.Location())
	if err != nil {
		return fmt.Errorf("error reading YAML file: %w", err)
	}

	if all {
		// Prepare hosts file entries
		for _, site := range headerSites.Sites {
			sites = append(sites, site)
			addNewGoroutine(site, combinedTime, isInBackground)
		}
	} else {
		sites = append(sites, specificSite)
		addNewGoroutine(specificSite, combinedTime, isInBackground)
	}

	// Update the hosts file with the new entries
	if err := updateHostsFile(sites); err != nil {
		return fmt.Errorf("error updating hosts file: %w", err)
	}

	return nil
}
func cleanupStrict(all bool, url string) error {
	hostsMu.Lock()         // Lock the mutex
	defer hostsMu.Unlock() // Ensure it gets unlocked at the end

	// Read etc/hosts file
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("error reading hosts file: %v", err)
	}

	if !all && url == "" {
		return fmt.Errorf("empty URL")
	}

	// Read sites from the specified YAML file
	var sites []string
	if all {
		headerSites, err := readConfig(absolutePathToSelfControl + "/configs/config.yaml")
		if err != nil {
			return err
		}

		// Prepare hosts file entries
		for _, site := range headerSites.Sites {
			sites = append(sites, site)
			removeGouroutine(site)
		}
	} else {
		sites = append(sites, url)
		removeGouroutine(url)
	}

	// Removing entries
	lines := strings.Split(string(content), "\n")
	var newLines []string
	removeExtraLine := false
	for _, line := range lines {
		if !all && strings.Contains(line, "# Added by selfcontrol") {
			newLines = append(newLines, line)
			continue
		}
		if !strings.Contains(line, "# Added by selfcontrol") {
			shouldKeep := true
			for i := 0; i < len(sites); {
				site := sites[i]
				if strings.Contains(line, site) {
					// Remove the site from the sites array
					sites = append(sites[:i], sites[i+1:]...) // Remove the matched site
					shouldKeep = false
					break
				} else {
					i++ // Only increment if no removal
				}
			}
			if shouldKeep {
				newLines = append(newLines, line)
			}
		} else {
			removeExtraLine = true
		}
	}

	if all && removeExtraLine {
		newLines = newLines[:len(newLines)-1]
	}
	// Write back to hosts file
	return os.WriteFile(hostsFile, []byte(strings.Join(newLines, "\n")), 0644)
}

// Function to remove site from the config.yaml file
func removeBlockedSiteFromConfig(site string) error {
	configFilePath := absolutePathToSelfControl + "/configs/config.yaml"

	// Read the current config
	config, err := readConfig(configFilePath)
	if err != nil {
		return err
	}

	found := false
	// Removing the site from the config
	for i, s := range config.Sites {
		if strings.EqualFold(s, site) { // Case insensitive comparison
			config.Sites = append(config.Sites[:i], config.Sites[i+1:]...) // Remove the site
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("site %s not found in the config", site)
	}
	// Write the updated data back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		return fmt.Errorf("failed to write updated config to file: %w", err)
	}

	fmt.Println("Successfully removed site:", site)
	return nil
}

// Function to check current mode of the application, returns mode in string format
func checkMode() (string, error) {
	config, err := readConfig(configFilePath)
	if err != nil {
		return "", err
	}
	return config.CurrentStatus.Mode, nil
}

// Switch mode from strict to normal and vice versa
func switchModeStrict(option int) error {
	configFilePath := absolutePathToSelfControl + "/configs/config.yaml"

	// Read the current config
	config, err := readConfig(configFilePath)
	if err != nil {
		return err
	}
	var mode string
	switch {
	case option == 1:
		mode = "strict"
	case option == 2:
		mode = "normal"
	}

	// Update the mode in the config
	config.CurrentStatus.Mode = mode

	// Write the updated data back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		return fmt.Errorf("failed to write updated config to file: %w", err)
	}

	fmt.Println("Successfully switched mode to:", mode)
	return nil
}

func backgroundBlocker2(startup bool) {
	fmt.Println("\n**********Background blocking2**********")
	// Remove everything from the hosts file first
	content, _ := os.ReadFile(hostsFile)
	removeBlockedSiteFromHostsFile(true, "", content)

	currentTime := time.Now()
	parsedTime, err := time.Parse(DateTimeLayout, currentTime.Format(DateTimeLayout))
	if err != nil {
		fmt.Printf("Error parsing time: %v\n", err)
		return
	}
	fmt.Println("Time started: ", parsedTime)
	var path string
	if startup {
		path = absolutePathToSelfControl + "/configs/blocked-sites.yaml"
		// Write the PID to selfcontrol.lock in tmp
		pid := os.Getpid()
		lockFilePath := absolutePathToSelfControl + "/tmp/selfcontrol.lock"
		if err := os.WriteFile(lockFilePath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
			fmt.Printf("Error writing PID to lock file: %v\n", err)
			return
		}
	} else {
		path = configFilePath
	}

	// Block all the sites in the config file
	blockSitesStrict(true, path, "", true)
	fmt.Println(goroutineContexts)
	wg.Wait()
	// Once all goroutines are done, cleanup all sites
	cleanup(true, "")
	fmt.Println("Background blocking completed")
}

func normalModeMenu() {
	fmt.Println("\n**********Normal Mode**********")
	fmt.Println("1. Block sites")
	fmt.Println("2. Unblock sites")
	fmt.Println("3. View blocked sites")
	fmt.Println("4. Exit")
	fmt.Printf("Enter your choice: ")
}
