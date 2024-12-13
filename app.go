package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	goroutineContexts = make(map[string]context.CancelFunc) // Map to hold cancel functions
	mu                sync.Mutex                            // Mutex to protect access to the map
	wg                sync.WaitGroup
)

// Header of yaml file with all sites
type HeaderSite struct {
	Sites []Site `yaml:"sites"`
}

// Site represents a single site to block
type Site struct {
	Name             string `yaml:"name"`
	URL              string `yaml:"url"`
	Duration         string `yaml:"duration"`
	CurrentlyBlocked bool   `yaml:"currentlyBlocked"`
}

// Header of yaml file with all schedules
type HeaderSchedule struct {
	Schedules []Schedule `yaml:"schedules"`
}

// Schedule represents a single schedule
type Schedule struct {
	Name      string   `yaml:"name"`
	Days      []string `yaml:"days"`
	StartTime string   `yaml:"startTime"`
	EndTime   string   `yaml:"endTime"`
}

// Function to add new goroutine when editing or adding to yaml file
func addNewGoroutine(url string, expiryTime time.Time) {
	ctx, cancel := context.WithCancel(context.Background()) // Create a new context for each site
	mu.Lock()
	goroutineContexts[url] = cancel // Use the site URL as the key
	mu.Unlock()
	go func(expiry time.Time, url string, ctx context.Context) {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done(): // Check if context is cancelled through the cancel() function in cleanup
				fmt.Printf("Goroutine for %s cancelled.\n", url)
				showMenu()
				return
			case <-ticker.C: // Counter to automatically remove site after expiry time
				if time.Now().After(expiry) {
					cleanup(false, url) // Replace with actual cleanup logic
					fmt.Printf("Cleaned up 1%s\n", url)
					showMenu()
					return
				}
			}
		}
	}(expiryTime, url, ctx)
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

// Function to write to yaml file
func writeToYamlFile(filename string, name string, url string, expiryTimeString string) error {
	// Read yaml file
	headerSites, err := readBlockedYamlFile(filename)
	formatted_url := FormatString(url)
	if err != nil {
		return err
	}

	// Check if site already exists
	for _, site := range headerSites.Sites {
		if site.URL == formatted_url {
			return fmt.Errorf("Site already blocked")
		}
	}

	// Add new site to yaml file
	newSite := Site{
		Name:             FormatString(name),
		URL:              formatted_url,
		Duration:         expiryTimeString,
		CurrentlyBlocked: false,
	}
	headerSites.Sites = append(headerSites.Sites, newSite)

	// Write to original file
	writeAndSave(filename, headerSites)

	return nil
}

// Function to create a new schedule and write in to yaml file
func writeToScheduleYamlFile(filename string, name string, days []string, startTime string, endTime string) error {
	headerSchedule, err := readScheduleYamlFile(filename)
	if err != nil {
		return err
	}

	newSchedule := Schedule{
		Name:      name,
		Days:      days,
		StartTime: startTime,
		EndTime:   endTime,
	}
	headerSchedule.Schedules = append(headerSchedule.Schedules, newSchedule)

	writeAndSave(filename, headerSchedule)
	return nil
}

// Function to delete site from yaml file
func deleteSiteFromYamlFile(filename string, name, url string) error {

	// Read yaml file
	headerSites, err := readBlockedYamlFile(filename)
	if err != nil {
		return err
	}

	// Remove site from yaml file
	var updatedSites []Site
	name = strings.TrimSpace(strings.ToLower(name))
	for _, site := range headerSites.Sites {
		if name != "" && site.Name != name {
			updatedSites = append(updatedSites, site)
		} else if site.URL != url {
			updatedSites = append(updatedSites, site)
		}
	}
	headerSites.Sites = updatedSites

	//Write and truncate original file
	writeAndSave(filename, headerSites)

	return nil
}

// Function to delete schedule from yaml file
func deleteScheduleFromYamlFile(filename string, name string) error {
	headerSchedule, err := readScheduleYamlFile(filename)
	if err != nil {
		return err
	}

	var updatedSchedules []Schedule
	for _, schedule := range headerSchedule.Schedules {
		if schedule.Name != name {
			updatedSchedules = append(updatedSchedules, schedule)
		}
	}
	headerSchedule.Schedules = updatedSchedules

	writeAndSave(filename, headerSchedule)
	return nil
}

// Function to update the expiry time for blocked sites
func updateExpiryTime(filename string, url string, newExpiryTime time.Time, alreadyExists bool) error {
	newExpiryTimeStr := newExpiryTime.Format("2006-01-02 15:04:05 -0700")
	sites, err := readBlockedYamlFile(filename)
	if err != nil {
		return err
	}
	if len(sites.Sites) == 0 {
		fmt.Println("No sites in config file")
		return nil
	}

	// Iterating through sites to find if requested site is blocked
	siteExists := false
	for i := range sites.Sites {
		if sites.Sites[i].URL == url {
			sites.Sites[i].Duration = newExpiryTimeStr
			siteExists = true
			break
		}
	}

	if !siteExists {
		fmt.Println("Site not found in config file")
		return nil
	}

	// Writing to original file
	writeAndSave(filename, sites)

	if alreadyExists { // bool to check if the site already exists in config, if it does, we need to update the goroutine. If it does not ie. startup, skip
		fmt.Printf("Updated expiry time for site: %s to %v", url, newExpiryTimeStr)
		cleanup(false, url)
		blockSites(false, filename, url, newExpiryTime)
	}
	return nil
}

// Fcunction to display the status of the blocked sites
func displayStatus(fileName string) {
	status, err := readBlockedYamlFile(fileName)
	if err != nil {
		return
	}
	empty := true
	for _, site := range status.Sites {
		parsedTime, err := time.Parse(DateTimeLayout, site.Duration)
		if err != nil {
			fmt.Println("Error parsing time:", err)
			return
		}

		if parsedTime.Before(time.Now()) || !site.CurrentlyBlocked {
			continue
		}
		empty = false
		timeDifference := time.Until(parsedTime)      // Get the duration between parsedTime and current time
		hours := int(timeDifference.Hours())          // Convert to hours
		minutes := int(timeDifference.Minutes()) % 60 // Convert to minutes
		seconds := int(timeDifference.Seconds()) % 60 // Convert to seconds and get the remainder

		fmt.Printf("- %-20s Time remaining: %d hours %d minutes and %d seconds\n", site.URL, hours, minutes, seconds)
		fmt.Printf("- %-20s Expiry Time: %s\n", site.URL, site.Duration)
	}
	if empty {
		fmt.Println("No sites are currently blocked")
	}
}

func showMenu() {
	fmt.Println("\n====== SelfControl Menu ======")
	fmt.Println("1. Start new block")
	fmt.Println("2. Show current status")
	fmt.Println("3. Edit expiry time for blocked sites")
	fmt.Println("4. Unblock all sites")
	fmt.Println("5. Exit")
	fmt.Println("6: Unblock all sites")
	fmt.Println("7: Unblock specific sites")
	fmt.Println("8: Add new site to block")
	fmt.Println("9: Delete site from yaml configuration")
	fmt.Println("10: Load schedule")
	fmt.Println("11: Add new schedule")
	fmt.Println("12: Delete schedule")
	fmt.Println("13: Edit schedule")
	fmt.Print("\nEnter your choice (1-13): ")
}

// Function to read user input
func readUserInput(reader *bufio.Reader) string {
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
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

// Function to block sites using the specified YAML file and update the /etc/hosts file
func blockSites(all bool, yamlFile string, specificSite string, expiryTime time.Time) error {
	var sites []string

	headerSites, err := readBlockedYamlFile(yamlFile)
	if err != nil {
		return fmt.Errorf("error reading YAML file: %w", err)
	}

	if all {
		// Read sites from the specified YAML file

		// Prepare hosts file entries
		for _, site := range headerSites.Sites {
			sites = append(sites, site.URL)
			editblockedStatusOnYamlFile(yamlFile, site.URL, true)
			addNewGoroutine(site.URL, expiryTime)
		}
	} else {
		sites = append(sites, specificSite)
		addNewGoroutine(specificSite, expiryTime)
		editblockedStatusOnYamlFile(yamlFile, specificSite, true)
	}

	// Update the hosts file with the new entries
	if err := updateHostsFile(sites); err != nil {
		return fmt.Errorf("error updating hosts file: %w", err)
	}

	return nil
}

// Function to update the hosts file
func updateHostsFile(sites []string) error {
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

// Function to edit blocked status on yaml file
func editblockedStatusOnYamlFile(filename string, url string, status bool) error {
	headerSites, err := readBlockedYamlFile(filename)
	if err != nil {
		return err
	}

	for i := range headerSites.Sites {
		if headerSites.Sites[i].URL == url {
			headerSites.Sites[i].CurrentlyBlocked = status
			break
		}
	}

	writeAndSave(filename, headerSites)

	return nil
}

// Function to edit schedules on yaml file
func editSchedulesonYamlFile(filename string, reader *bufio.Reader) error {
	fmt.Print("Enter name of schedule to edit: ")
	name := readUserInput(reader)
	fmt.Print("Enter option to edit(1: Edit name 2: Edit days 3: Edit start time 4: Edit end time): ")
	option := readUserInput(reader)
	var field string
	if option == "3" || option == "4" {
		field = queryForTime(reader, option == "3")
	} else {
		fmt.Print("Enter field to edit: ")
		field = readUserInput(reader)
	}
	headerSchedule, err := readScheduleYamlFile(filename)
	if err != nil {
		return err
	}

	validSchedule := false
outer:
	for i := range headerSchedule.Schedules {
		switch option {
		case "1":
			if headerSchedule.Schedules[i].Name == name {
				headerSchedule.Schedules[i].Name = field
				validSchedule = true
				break outer
			}
		case "2":
			if headerSchedule.Schedules[i].Name == name {
				headerSchedule.Schedules[i].Days = strings.Split(field, ",")
				validSchedule = true
				break outer
			}
		case "3":
			if headerSchedule.Schedules[i].Name == name {
				if err := checkStartBeforeEnd(field, headerSchedule.Schedules[i].EndTime); err != nil {
					fmt.Println("Error checking start time before end time:", err)
					break outer
				}
				headerSchedule.Schedules[i].StartTime = field
				validSchedule = true
			}
		case "4":
			if headerSchedule.Schedules[i].Name == name {
				if err := checkStartBeforeEnd(headerSchedule.Schedules[i].StartTime, field); err != nil {
					fmt.Println("Error checking start time before end time:", err)
					break outer
				}
				headerSchedule.Schedules[i].EndTime = field
				validSchedule = true
			}
		}

	}
	if validSchedule {
		writeAndSave(filename, headerSchedule)
		fmt.Println("Schedule edited successfully")
	} else {
		return fmt.Errorf("Schedule not able to be edited")
	}
	return nil
}

// Removes all entries inside etc/hosts that were added by selfcontrol
func cleanup(all bool, url string) error {
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
		headerSites, err := readBlockedYamlFile("blocked-sites.yaml")
		if err != nil {
			return err
		}

		// Prepare hosts file entries
		for _, site := range headerSites.Sites {
			sites = append(sites, site.URL)
			editblockedStatusOnYamlFile("blocked-sites.yaml", site.URL, false)
			removeGouroutine(site.URL)
		}
	} else {
		sites = append(sites, url)
		editblockedStatusOnYamlFile("blocked-sites.yaml", url, false)
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

// Function to block sites based on schedule in yaml file
func loadSchedule(data HeaderSchedule, name string, currentTime time.Time) {
	for _, schedule := range data.Schedules {
		if schedule.Name == name {
			fmt.Printf("Found schedule %s\n", name)
			for _, day := range schedule.Days {
				if strings.ToLower(currentTime.Weekday().String()) == day && currentTime.Format("15:04") >= schedule.StartTime && currentTime.Format("15:04") <= schedule.EndTime {
					fmt.Println("Today is Thursday. Time to block sites!")
					endTime, err := time.Parse("15:04", schedule.EndTime)
					if err != nil {
						fmt.Printf("Error parsing end time: %v\n", err)
						break
					}
					finalEndTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), endTime.Hour(), endTime.Minute(), endTime.Second(), 0, currentTime.Location())
					blockSites(true, "blocked-sites.yaml", "", finalEndTime)
					var headerSite HeaderSite
					headerSite, err = readBlockedYamlFile("blocked-sites.yaml")
					if err != nil {
						fmt.Printf("Error reading blocked sites: %v\n", err)
					}
					for _, site := range headerSite.Sites {
						updateExpiryTime("blocked-sites.yaml", site.URL, finalEndTime, false)
						editblockedStatusOnYamlFile("blocked-sites.yaml", site.URL, true)
					}
					return
				}
			}
			fmt.Println("Not time to block sites")
		}
	}
}

// Function to repeatedly ask for valid time input
func queryForTime(reader *bufio.Reader, startTime bool) string {
	var time string
	for {
		if startTime {
			fmt.Print("Enter start time: ")
		} else {
			fmt.Print("Enter end time: ")
		}
		rawTime := readUserInput(reader)
		var err error
		time, err = FormatTime(rawTime)
		if err != nil {
			fmt.Println(err)
			continue
		} else {
			break
		}
	}
	return time
}

// Function to remove goroutine for a specific site
func removeGouroutine(url string) {
	mu.Lock()
	wg.Add(1)
	if cancel, exists := goroutineContexts[url]; exists { //accessing the goroutine map to find the correct cancel() function for the url
		cancel() // Cancelling the goroutine using the cancel function found in the map
		delete(goroutineContexts, url)
		fmt.Printf("\nCancelled goroutine for site: %s\n", url)
	}
	wg.Done()
	mu.Unlock()
	wg.Wait()
}

func main() {

	reader := bufio.NewReader(os.Stdin)

	for {
		wg.Wait()
		showMenu()
		choice := readUserInput(reader)
		switch choice {
		case "1":
			fmt.Println("Chosen to block sites")

			sitesFileLocation := "blocked-sites.yaml"
			duration := getDuration(reader)

			// Calculate expiry time
			expiryTime := time.Now().Add(duration)
			fmt.Print("Expiry Time: ", expiryTime)

			// Block sites
			if err := blockSites(true, sitesFileLocation, "", expiryTime); err != nil {
				fmt.Printf("Error blocking sites: %v\n", err)
				continue
			}
			// Manually activating gourutine for init
			headerSites, err := readBlockedYamlFile(sitesFileLocation)
			if err != nil {
				fmt.Printf("Error reading YAML file: %v\n", err)
				continue
			}
			for _, site := range headerSites.Sites {
				updateExpiryTime(sitesFileLocation, site.URL, expiryTime, false)
			}

		case "2":
			fmt.Println("Chosen to show current status")
			displayStatus("blocked-sites.yaml")
		case "3":
			fmt.Print("Enter which site to change expiry time")
			site := readUserInput(reader)
			fmt.Print("Enter new expiry time")
			newExpiryTime := time.Now().Add(getDuration(reader))
			if err := updateExpiryTime("blocked-sites.yaml", site, newExpiryTime, true); err != nil {
				fmt.Printf("Error updating expiry time: %v\n", err)
			}
		case "5":
			fmt.Println("Goodbye!")
			cleanup(true, "")
			return
		case "6": // Unblock all sites
			fmt.Println("Unblocked all sites")
			cleanup(true, "")
		case "7": // Unblock specific site
			fmt.Print("Enter site to unblock: ")
			site := readUserInput(reader)
			if err := cleanup(false, site); err != nil {
				fmt.Printf("Error unblocking site: %v\n", err)
				continue
			}
			fmt.Println("Unblocked site: ", site)
		case "8": // Add new site to block
			fmt.Print("Enter site URL: ")
			site := readUserInput(reader)
			fmt.Print("Enter blocking duration: ")
			duration := readUserInput(reader)
			parsedDuration, err := time.ParseDuration(duration)
			if err != nil {
				fmt.Printf("Invalid duration format: %v\n", err)
				continue
			}
			expiryTime := time.Now().Add(parsedDuration)
			name := GetNameFromURL(site)
			formattedExpiryTime := expiryTime.Format("2006-01-02 15:04:05 -0700")
			fmt.Print("Expiry Time: ", formattedExpiryTime)
			writeToYamlFile("blocked-sites.yaml", name, site, formattedExpiryTime)
			blockSites(false, "blocked-sites.yaml", site, expiryTime)
		case "9": // Delete site from yaml configuration
			fmt.Print("Enter site to delete from Config: ")
			site := readUserInput(reader)
			cleanup(false, site)
			if err := deleteSiteFromYamlFile("blocked-sites.yaml", "", site); err != nil {
				fmt.Printf("Error deleting site: %v\n", err)
			}
		case "10":
			currentTime := time.Now()
			headerSchedule, err := readScheduleYamlFile("schedules.yaml")
			if err != nil {
				fmt.Printf("Error reading schedule file: %v\n", err)
				continue
			}
			fmt.Print("Enter name of schedule: ")
			name := readUserInput(reader)
			loadSchedule(headerSchedule, name, currentTime)

		case "11":
			createNewSchedule(reader)
		case "12":
			fmt.Print("Enter name of schedule to delete: ")
			name := readUserInput(reader)
			if err := deleteScheduleFromYamlFile("schedules.yaml", name); err != nil {
				fmt.Printf("Error deleting schedule: %v\n", err)
			}

		case "13":
			if err := editSchedulesonYamlFile("schedules.yaml", reader); err != nil {
				fmt.Printf("Error editing schedule: %v\n", err)
				continue
			}

		default:
			fmt.Println("Invalid choice. Please try again.")
		}
	}
}
