package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
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
	StartedAt       string `yaml:"started_at"`
	EndedAt         string `yaml:"ended_at"`
	Mode            string `yaml:"mode"`
	BlockOnRestart  string `yaml:"block_on_restart"`
	BlockCustomTime string `yaml:"block_custom_time"`
}

type TimeRange struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

// Function to read the config file
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
					fmt.Println("\n*****Sites that are currently blocked*****")
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

// Function to return the expirytime of the config yaml if it is in a schedule
func getExpiryTime() (string, error) {
	configs, err := readConfig(configFilePath)
	if err != nil {
		return "", err
	}
	currentDay := strings.ToLower(time.Now().Weekday().String()) // Get the current day of the week
	currentTime := time.Now().Format("15:04")                    // Get current time in HH:MM format

	// Check if there are schedules for the current day
	if schedules, exists := configs.Schedules[currentDay]; exists {
		for _, schedule := range schedules {
			// Check if the current time falls within any of the scheduled time ranges
			if isTimeInRange(currentTime, schedule.Start, schedule.End) {
				return schedule.End, nil // Exit after printing sites for the current time range
			}
		}
		return "", fmt.Errorf("no sites are being blocked now") // Return an error if no sites were found
	} else {
		// Return an error if no schedules were found or no sites to block
		return "", fmt.Errorf("no schedules found for %s", currentDay)
	}
}

// Function to check if the current time is within the specified time range
func isTimeInRange(currentTime, start, end string) bool {
	current, _ := time.Parse("15:04", currentTime)
	startTime, _ := time.Parse("15:04", start)
	endTime, _ := time.Parse("15:04", end)

	return current.After(startTime) && current.Before(endTime)
}

func blockSitesCustomTime(yamlFile string, isInBackground bool, expiryTime time.Time) error {
	var sites []string
	// Read sites from the specified YAML file
	headerSites, err := readConfig(yamlFile)
	if err != nil {
		return fmt.Errorf("error reading YAML file: %w", err)
	}

	fmt.Println("Blocking sites until", expiryTime.Format("15:04:05"))

	// Prepare hosts file entries
	sites = append(sites, headerSites.Sites...)
	addNewGoroutine("combined", expiryTime, isInBackground) // Pass the expiryTime directly
	editEndingTime(expiryTime.Format(DateTimeLayout))
	editBlockCustomTimeStatus("true")

	// Update the hosts file with the new entries
	if err := updateHostsFile(sites); err != nil {
		return fmt.Errorf("error updating hosts file: %w", err)
	}

	fmt.Println("Following sites are blocked until", expiryTime.Format("15:04:05"))
	for _, site := range sites {
		fmt.Println("- ", site)
	}
	return nil
}

// Function that blocks all sites in config yaml if it is during schedule time
func blockSitesStrict(yamlFile string, isInBackground bool) error {
	var sites []string
	// Read sites from the specified YAML file
	headerSites, err := readConfig(yamlFile)
	if err != nil {
		return fmt.Errorf("error reading YAML file: %w", err)
	}
	expiryTime, errSites := printSitesToBlock(headerSites, false)
	if errSites != nil {
		return fmt.Errorf("no sites to block")
	}
	fmt.Println("Blocking sites until", expiryTime)
	currentDate := time.Now()
	expiryTimeParsed, _ := time.Parse("15:04", expiryTime)
	combinedTime := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(),
		expiryTimeParsed.Hour(), expiryTimeParsed.Minute(), 0, 0, currentDate.Location())

	// Prepare hosts file entries
	sites = append(sites, headerSites.Sites...)
	addNewGoroutine("combined", combinedTime, isInBackground)

	// Remove custom time blocking status
	editBlockCustomTimeStatus("false")
	// Update the hosts file with the new entries
	if err := updateHostsFile(sites); err != nil {
		return fmt.Errorf("error updating hosts file: %w", err)
	}

	return nil
}

// Function to get ending time for custom time in yaml
func getEndingTime() (string, error) {
	config, err := readConfig(configFilePath)
	if err != nil {
		return "", err
	}
	return config.CurrentStatus.EndedAt, nil
}

// Function to edit ending time for custom time in yaml
func editEndingTime(newEndingTime string) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	// Update the ending_at field in current_status
	config.CurrentStatus.EndedAt = newEndingTime

	// Write the updated configuration back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}
}

// Function to get custom time blocking status in yaml
func getBlockCustomTimeStatus() (string, error) {
	config, err := readConfig(configFilePath)
	if err != nil {
		return "", err
	}
	return config.CurrentStatus.BlockCustomTime, nil
}

// Function to change blockcustomtime status in yaml true or false
func editBlockCustomTimeStatus(status string) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}
	config.CurrentStatus.BlockCustomTime = status
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing to config file: %v\n", err)
		return
	}
	fmt.Println("Block custom time status changed successfully")
}

// Function to remove all goroutines and cleanup the hosts file, sets blockcustomtime status to false
func cleanupStrict() error {
	hostsMu.Lock()         // Lock the mutex
	defer hostsMu.Unlock() // Ensure it gets unlocked at the end

	// Read etc/hosts file
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("error reading hosts file: %v", err)
	}

	// Read sites from the specified YAML file
	var sites []string
	// Prepare hosts file entries
	headerSites, err := readConfig(absolutePathToSelfControl + "/configs/config.yaml")
	if err != nil {
		return err
	}
	sites = append(sites, headerSites.Sites...)
	removeGouroutine("combined")

	editBlockCustomTimeStatus("false")

	removeline := false
	// Removing entries
	lines := strings.Split(string(content), "\n")
	var newLines []string
	for _, line := range lines {
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
			removeline = true
		}
	}

	if removeline {
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

// run only if goroutine map not empty on exit or during startup
func backgroundBlocker(startup bool) {
	fmt.Println("\n**********Background blocking**********")
	// Remove everything from the hosts file first
	content, _ := os.ReadFile(hostsFile)
	removeBlockedSiteFromHostsFile(true, content)

	currentTime := time.Now()
	parsedTime, err := time.Parse(DateTimeLayout, currentTime.Format(DateTimeLayout))
	if err != nil {
		fmt.Printf("Error parsing time: %v\n", err)
		return
	}
	fmt.Println("Time started: ", parsedTime)
	var path string
	if startup {
		path = absolutePathToSelfControl + "/configs/config.yaml"
		// Write the PID to selfcontrol.lock in tmp
		pid := os.Getpid()
		lockFilePath := absolutePathToSelfControl + "/tmp/selfcontrol.lock"
		if err := os.WriteFile(lockFilePath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
			fmt.Printf("Error writing PID to lock file: %v\n", err)
			return
		}
	} else {
		path = absolutePathToSelfControl + "/configs/config.yaml"
	}

	// Check if it is blocking based on time or schedule
	status, err := getBlockCustomTimeStatus()
	if err != nil {
		fmt.Printf("Error getting block custom time status: %v\n", err)
		return
	}
	if status == "true" {
		endTime, errEndTime := getEndingTime()
		if errEndTime != nil {
			fmt.Printf("Error getting ending time: %v\n", errEndTime)
			return
		}
		parsedEndTime, err := time.Parse("2006-01-02 15:04:05 -0700", endTime)
		if err != nil {
			fmt.Printf("Error parsing end time: %v\n", err)
			return
		}
		blockSitesCustomTime(path, true, parsedEndTime)
	} else {
		// Block all the sites in the config file
		blockerr := blockSitesStrict(path, true)
		if blockerr != nil {
			fmt.Printf("Error blocking sites: %v\n", blockerr)
			fmt.Println("Background blocking completed")
			return
		}
	}
	fmt.Println(goroutineContexts)
	wg.Wait()
	// Once all goroutines are done, cleanup all sites
	cleanupStrict()
	fmt.Println("Background blocking completed")
}

func addNewSiteToConfig(site string) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}

	for _, existingSite := range config.Sites {
		if strings.EqualFold(existingSite, site) { // Case insensitive comparison
			fmt.Println("This site is already in the configuration.")
			return
		}
	}

	config.Sites = append(config.Sites, site)
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing to config file: %v\n", err)
		return
	}
	fmt.Println("Site added successfully")
}
func normalModeMenu() {
	time.Sleep(210 * time.Millisecond)
	fmt.Println("\n**********Normal Mode**********")
	fmt.Println("1. Enter new site to block and block all sites")
	fmt.Println("2. Unblock sites")
	fmt.Println("3. Remove site from block list and unblock")
	fmt.Println("4. Exit")
	fmt.Printf("Enter your choice: ")
}

func queryForSchedule(reader *bufio.Reader) string {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return ""
	}
	fmt.Println("Select a site to remove:")
	for i, site := range config.Sites {
		fmt.Printf("%d: %s\n", i+1, site) // Display sites with numbers
	}
	fmt.Print("Enter your choice: ")
	choice := readUserInput(reader)
	index, err := strconv.Atoi(choice)
	if err != nil || index < 1 || index > len(config.Sites) {
		fmt.Println("Invalid choice. Please enter a valid number.")
		return ""
	}
	return config.Sites[index-1]
}

func deleteSiteFromConfig(reader *bufio.Reader) {
	site := queryForSchedule(reader)
	removeBlockedSiteFromConfig(site)
}
func deleteAndUnblockSiteFromConfig(reader *bufio.Reader) {
	site := queryForSchedule(reader)
	cleanupStrict()
	removeBlockedSiteFromConfig(site)
	blockSitesStrict(configFilePath, false)
}

// Function to get block on restart status in yaml
func getBlockOnRestartStatus() (string, error) {
	config, err := readConfig(configFilePath)
	if err != nil {
		return "", err
	}
	return config.CurrentStatus.BlockOnRestart, nil
}
func changeBlockOnRestartStatus(status string) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}
	config.CurrentStatus.BlockOnRestart = status
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing to config file: %v\n", err)
		return
	}
	fmt.Println("Block on restart status changed successfully")
}

// Function that checks strict mode and blocks access if it is
func accessingMenusInStrictMode() bool {
	mode, err := checkMode()
	if err != nil {
		fmt.Println("Error checking mode:", err)
		return false
	}

	if mode == "strict" {
		var expiryTime string

		status, err := getBlockCustomTimeStatus()
		if err == nil && status == "true" {
			expiryTime, err = getEndingTime()
		} else {
			expiryTime, err = getExpiryTime()
		}

		if err != nil {
			fmt.Println("Error getting expiry time:", err)
			return false
		}

		fmt.Printf("Cannot access this menu while in Strict Mode. Expires at: %s\n", expiryTime)
		return false
	}

	// If not in strict mode, return true to allow access.
	return true
}

func switchingStrictModeMenu(reader *bufio.Reader) {
	fmt.Println("Select an option:")
	fmt.Println("1: Turn on strict mode")
	fmt.Println("2: Turn off strict mode")
	fmt.Printf("Enter choice: ")
	choice := readUserInput(reader)

	if choice == "1" {
		fmt.Println("You are about to enter strict mode and will not be able to remove blocks until after all blocks have timed out. Are you sure?")
		fmt.Println("1: Yes")
		fmt.Println("2: No")
		fmt.Printf("Enter choice: ")
		confirm := readUserInput(reader)
		if confirm == "1" {
			switchModeStrict(1)
		}
	} else if choice == "2" {
		// Check if any sites are currently being blocked in strict mode
		status := len(goroutineContexts) > 0

		if status {
			fmt.Println("Cannot turn off strict mode because sites are currently being blocked.")
		} else {
			fmt.Println("You are about to turn off strict mode. Are you sure?")
			fmt.Println("1: Yes")
			fmt.Println("2: No")
			fmt.Printf("Enter choice: ")
			confirm := readUserInput(reader)
			if confirm == "1" {
				switchModeStrict(2) // Assuming 0 is for normal mode
			}
		}
	} else {
		fmt.Println("Invalid choice. Please try again.")
	}
}
