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

// Function to write to the hosts file
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

// Function to delete a site from the config file
func removeBlockedSiteFromHostsFile(all bool, content []byte) error {
	var sites []string
	if all {
		headerSites, err := readConfig(configFilePath)
		if err != nil {
			return err
		}

		// Prepare hosts file entries
		sites = append(sites, headerSites.Sites...)
	}
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

// Functions for config file

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

// Function to display schedule details in config
func displaySchedule() {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	fmt.Println("\n*** Schedule for Blocking Sites ***")
	for day, schedules := range config.Schedules {
		fmt.Printf("%s:\n", strings.ToUpper(day))
		for i, schedule := range schedules {
			fmt.Printf("  %d: Start: %s, End: %s\n", i+1, schedule.Start, schedule.End)
		}
	}
}

// Function to add day to schedule
func addDayToSchedule(reader *bufio.Reader) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	// Prompt for the new day to add and check if its a valid day
	fmt.Print("Enter the day you want to add (e.g., 'saturday'): ")
	newDay := FormatString(readUserInput(reader))
	validDay := false
	for _, day := range daysOfWeek {
		if strings.EqualFold(newDay, day) {
			validDay = true
			break
		}
	}
	if !validDay {
		fmt.Println("Invalid day entered.")
		return
	}
	// Check if the day already exists
	if _, exists := config.Schedules[newDay]; exists {
		fmt.Println("This day already exists in the schedule.")
		return
	}

	// Initialize a slice to hold time ranges
	timeRanges := []TimeRange{}

	// Loop to add time ranges
	for {
		var startTime, endTime string

		// Get start time
		fmt.Print("Enter start time (HH:MM): ")
		startTime = readUserInput(reader)
		startTime, err = FormatTime(startTime)
		if err != nil {
			fmt.Println("Error formatting time: ", err)
			return
		}

		// Get end time
		fmt.Print("Enter end time (HH:MM): ")
		endTime = (readUserInput(reader))

		endTime, err = FormatTime(endTime)
		if err != nil {
			fmt.Println("Error formatting time: ", err)
			return
		}

		err := checkStartBeforeEnd(startTime, endTime)
		if err != nil {
			fmt.Println("Error: ", err)
			return
		}
		// Append the new time range to the slice
		timeRanges = append(timeRanges, TimeRange{Start: startTime, End: endTime})

		// Ask if the user wants to add another time range
		fmt.Print("Do you want to add another time range? (yes/no): ")
		another := FormatString(readUserInput(reader))
		if strings.ToLower(another) != "yes" {
			break
		}
	}

	// Add the new day and its time ranges to the schedule
	config.Schedules[newDay] = timeRanges

	// Write the updated configuration back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Successfully added %s with its time ranges to the schedule.\n", newDay)
}

// Function to delete day from schedule
func deleteDayFromSchedule(reader *bufio.Reader) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	fmt.Println("Available days in the schedule:")
	for day := range config.Schedules {
		fmt.Println(day)
	}
	// Prompt for the day to delete and check if it's a valid day
	fmt.Print("Enter the day you want to delete (e.g., 'saturday'): ")
	dayToDelete := FormatString(readUserInput(reader))

	// Check if the day exists
	if _, exists := config.Schedules[dayToDelete]; !exists {
		fmt.Println("This day does not exist in the schedule.")
		return
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the schedule for %s? (yes/no): ", dayToDelete)
	confirmation := FormatString(readUserInput(reader))
	if strings.ToLower(confirmation) != "yes" {
		fmt.Println("Deletion canceled.")
		return
	}

	// Delete the day from the schedule
	delete(config.Schedules, dayToDelete)

	// Write the updated configuration back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Successfully deleted the schedule for %s.\n", dayToDelete)
}

// Function to add time range for day in schedule
func addTimeRangeForDay(reader *bufio.Reader) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	for day := range config.Schedules {
		fmt.Println(day)
	}
	// Prompt for the day to add a time range
	fmt.Print("Enter the day you want to add a time range to (e.g., 'monday'): ")
	selectedDay := FormatString(readUserInput(reader))

	// Check if the selected day exists
	if _, exists := config.Schedules[selectedDay]; !exists {
		fmt.Println("Invalid day selected.")
		return
	}

	// Get start time
	fmt.Print("Enter start time (HH:MM): ")
	startTime := FormatString(readUserInput(reader))
	startTimeFormatted, err := FormatTime(startTime)
	if err != nil {
		fmt.Println("Error formatting time: ", err)
		return
	}

	// Get end time
	fmt.Print("Enter end time (HH:MM): ")
	endTime := FormatString(readUserInput(reader))

	// Validate end time format
	endTimeFormatted, err := FormatTime(endTime)
	if err != nil {
		fmt.Println("Error formatting time: ", err)
		return
	}

	errCheck := checkStartBeforeEnd(startTimeFormatted, endTimeFormatted)
	if errCheck != nil {
		fmt.Println("Error: ", errCheck)
		return
	}

	// Append the new time range to the selected day's schedule
	config.Schedules[selectedDay] = append(config.Schedules[selectedDay], TimeRange{Start: startTimeFormatted, End: endTimeFormatted})

	// Write the updated configuration back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Successfully added time range for %s: %s to %s.\n", selectedDay, startTime, endTime)
}

// Function to delete time range for day in schedule
func deleteTimeRangeForDay(reader *bufio.Reader) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	fmt.Println("Available days in the schedule:")
	for day := range config.Schedules {
		fmt.Println(day)
	}

	// Prompt for the day to delete a time range
	fmt.Print("Enter the day you want to delete a time range from (e.g., 'monday'): ")
	selectedDay := FormatString(readUserInput(reader))

	// Check if the selected day exists
	if _, exists := config.Schedules[selectedDay]; !exists {
		fmt.Println("Invalid day selected.")
		return
	}

	// Display the current time ranges for the selected day
	fmt.Printf("Current time ranges for %s:\n", selectedDay)
	for i, schedule := range config.Schedules[selectedDay] {
		fmt.Printf("%d: Start: %s, End: %s\n", i+1, schedule.Start, schedule.End)
	}

	// Get user choice for which time range to delete
	fmt.Print("Enter the index of the time range you want to delete: ")
	choice := FormatString(readUserInput(reader))
	selectedIndex, err := strconv.Atoi(choice)
	if err != nil || selectedIndex < 1 || selectedIndex > len(config.Schedules[selectedDay]) {
		fmt.Println("Invalid index selected.")
		return
	}

	// Remove the selected time range
	scheduleIndex := selectedIndex - 1 // Convert to zero-based index
	config.Schedules[selectedDay] = append(config.Schedules[selectedDay][:scheduleIndex], config.Schedules[selectedDay][scheduleIndex+1:]...)

	// Write the updated configuration back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Successfully deleted time range for %s.\n", selectedDay)
}

// Function to edit time range for day in schedule
func editTimeforSchedule(start bool, reader *bufio.Reader) {
	config, err := readConfig(configFilePath)
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	// Display the days with schedules
	fmt.Println("Available days in the schedule:")
	for day := range config.Schedules {
		fmt.Printf(" %s\n", day)

	}
	// Get user choice for the day
	fmt.Print("Enter the day you want to edit: ")
	selectedDay := FormatString(readUserInput(reader))

	// Check if the selected day exists
	if _, exists := config.Schedules[selectedDay]; !exists {
		fmt.Println("Invalid day selected.")
		return
	}

	// Display the start and end times for the selected day
	fmt.Printf("Schedules for %s:\n", selectedDay)
	for i, schedule := range config.Schedules[selectedDay] {
		fmt.Printf("%d: Start: %s, End: %s\n", i+1, schedule.Start, schedule.End)
	}

	// Get user choice for which time to edit
	fmt.Print("Enter the index of the schedule you want to edit: ")
	choice := FormatString(readUserInput(reader))
	selectedIndex, err := strconv.Atoi(choice)
	if err != nil || selectedIndex < 1 || selectedIndex > len(config.Schedules[selectedDay]) {
		fmt.Println("Invalid index selected.")
		return
	}
	var timeType string
	if start {
		timeType = "start"
	} else {
		timeType = "end"
	}
	// Get the new time
	fmt.Print("Enter new time in 24H format (HH:MM): ")
	unformattedtime := readUserInput(reader)
	newTime, err := FormatTime(unformattedtime)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	// Update the schedule based on user choice
	scheduleIndex := selectedIndex - 1 // Convert to zero-based index
	if timeType == "start" {
		err := checkStartBeforeEnd(newTime, config.Schedules[selectedDay][scheduleIndex].End)
		if err != nil {
			fmt.Println("Error: ", err)
			return
		}
		config.Schedules[selectedDay][scheduleIndex].Start = newTime

	} else if timeType == "end" {
		err := checkStartBeforeEnd(config.Schedules[selectedDay][scheduleIndex].Start, newTime)
		if err != nil {
			fmt.Println("Error: ", err)
			return
		}
		config.Schedules[selectedDay][scheduleIndex].End = newTime
	} else {
		fmt.Println("Invalid option. Please enter 'start' or 'end'.")
		return
	}

	// Write the updated configuration back to the YAML file
	if err := writeAndSave(configFilePath, config); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Successfully updated the %s time for %s to %s.\n", timeType, selectedDay, newTime)
}

// Function to remove site from the config.yaml file
func removeBlockedSiteFromConfig(site string) error {

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

// Function to remove site from config
func deleteSiteFromConfig(reader *bufio.Reader) {
	site := queryForSchedule(reader)
	removeBlockedSiteFromConfig(site)
}

// Function to remove site from config and unblock
func deleteAndUnblockSiteFromConfig(reader *bufio.Reader) {
	site := queryForSchedule(reader)
	status, err := getBlockCustomTimeStatus()
	if status == "true" {
		if err != nil {
			fmt.Printf("Error getting block status: %v\n", err)
			return // Exit or handle the error as needed
		}

		// If blocking is active, get the ending time
		endTime, err := getEndingTime()
		if err != nil {
			fmt.Println("Error getting ending time:", err)
			return
		}

		// Parse the ending time
		parsedEndTime, err := time.Parse(DateTimeLayout, endTime)
		if err != nil {
			fmt.Println("Error parsing end time:", err)
			return
		}
		cleanupStrict()
		removeBlockedSiteFromConfig(site)
		blockSitesCustomTime(configFilePath, false, parsedEndTime)
	} else {
		cleanupStrict()
		removeBlockedSiteFromConfig(site)
		blockSitesStrict(configFilePath, false)
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

// Function to check current mode of the application, returns mode in string format
func checkMode() (string, error) {
	config, err := readConfig(configFilePath)
	if err != nil {
		return "", err
	}
	return config.CurrentStatus.Mode, nil
}

// Function to add new site to config
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

// Function to get block on restart status in yaml
func getBlockOnRestartStatus() (string, error) {
	config, err := readConfig(configFilePath)
	if err != nil {
		return "", err
	}
	return config.CurrentStatus.BlockOnRestart, nil
}

// Function to edit block custom time status in yaml
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

// Function for etc/hosts
func printAllSites() {
	// Define the path to the hosts file
	hostsFilePath := "/etc/hosts" // Adjust this path if necessary

	// Read the contents of the hosts file
	content, err := os.ReadFile(hostsFilePath)
	if err != nil {
		fmt.Printf("Error reading hosts file: %v\n", err)
		return
	}

	// Split the content into lines
	lines := strings.Split(string(content), "\n")
	fmt.Println("Blocked Sites:")

	// Flag to indicate whether we're in the relevant section
	inSelfControlSection := false

	for _, line := range lines {
		// Check for the start of the self-control section
		if strings.Contains(line, "# Added by selfcontrol") {
			inSelfControlSection = true
			continue // Skip the line with the comment
		}

		// If we are in the self-control section, print the sites
		if inSelfControlSection {
			// Ignore comments and empty lines
			if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
				continue
			}

			// Split the line into parts
			parts := strings.Fields(line)
			if len(parts) > 1 && parts[0] == "127.0.0.1" {
				// The second part is the URL to block
				fmt.Println("- " + parts[1])
			}
		}
	}
}
