package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	goroutineContexts = make(map[string]context.CancelFunc) // Map to hold cancel functions
	mu                sync.Mutex                            // Mutex to protect access to the map
	wg                sync.WaitGroup                        // WaitGroup to wait for all goroutines to finish, specifically for background running
	wgRemove          sync.WaitGroup                        // WaitGroup for main function to wait for goroutines to be removed
	hostsMu           sync.Mutex                            // Mutex to protect hosts file operations
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

// Fcunction to display the status of the blocked sites
func displayStatus(fileName string) {
	status, err := readBlockedYamlFile(fileName)
	if err != nil {
		return
	}
	empty := true
	fmt.Println("\n***Blocked sites***")
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

// Function to show schedules from yaml file
func showSchedules(filepath string) {
	schedule, err := readScheduleYamlFile(filepath)
	if err != nil {
		fmt.Println("Error reading schedule file: ", err)
		return
	}
	for i, s := range schedule.Schedules {
		fmt.Println("***Schedule ", i+1, " ***")
		printScheduleInfo(s)
	}
}

func showMenu() {
	time.Sleep(200 * time.Millisecond)
	fmt.Println("\n\n ****Self Control Menu****")
	fmt.Println("1. Block all sites")
	fmt.Println("2. Show current status")
	fmt.Println("3. Add new site to block")
	fmt.Println("4. Edit blocked site duration")
	fmt.Println("5. Delete site from Config")
	fmt.Println("6. Show schedules")
	fmt.Println("7. Load schedule")
	fmt.Println("8. Create new Schedule")
	fmt.Println("9. Delete schedule")
	fmt.Println("10. Edit schedule")
	fmt.Println("11. Change password")
	fmt.Println("12. Unblock all sites")
	fmt.Println("13. Unblock specific site")
	fmt.Println("14. Exit")
	fmt.Println("15. Start in background")
	fmt.Print("\nChoose an option: ")
}

// Function to read user input
func readUserInput(reader *bufio.Reader) string {
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// Function to block sites using the specified YAML file and update the /etc/hosts file
func blockSites(all bool, yamlFile string, specificSite string, expiryTime time.Time, isInBackground bool) error {
	var sites []string

	// Read sites from the specified YAML file
	headerSites, err := readBlockedYamlFile(yamlFile)
	if err != nil {
		return fmt.Errorf("error reading YAML file: %w", err)
	}

	if all {

		// Prepare hosts file entries
		for _, site := range headerSites.Sites {
			sites = append(sites, site.URL)
			editblockedStatusOnYamlFile(yamlFile, site.URL, true)
			addNewGoroutine(site.URL, expiryTime, isInBackground)
		}
	} else {
		sites = append(sites, specificSite)
		addNewGoroutine(specificSite, expiryTime, isInBackground)
		editblockedStatusOnYamlFile(yamlFile, specificSite, true)
	}

	// Update the hosts file with the new entries
	if err := updateHostsFile(sites); err != nil {
		return fmt.Errorf("error updating hosts file: %w", err)
	}

	return nil
}

// Removes entries inside etc/hosts that were added by selfcontrol
func cleanup(all bool, url string) error {
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
		headerSites, err := readBlockedYamlFile(absolutePathToSelfControl + "/configs/blocked-sites.yaml")
		if err != nil {
			return err
		}

		// Prepare hosts file entries
		for _, site := range headerSites.Sites {
			sites = append(sites, site.URL)
			editblockedStatusOnYamlFile(absolutePathToSelfControl+"/configs/blocked-sites.yaml", site.URL, false)
			removeGouroutine(site.URL)
		}
	} else {
		sites = append(sites, url)
		if err := editblockedStatusOnYamlFile(blockedSitesFilePath, url, false); err != nil {
			return err
		}
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
					fmt.Printf("Block is in effect until %s!\n", schedule.EndTime)
					endTime, err := time.Parse("15:04", schedule.EndTime)
					if err != nil {
						fmt.Printf("Error parsing end time: %v\n", err)
						break
					}
					finalEndTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), endTime.Hour(), endTime.Minute(), endTime.Second(), 0, currentTime.Location())
					blockSites(true, blockedSitesFilePath, "", finalEndTime, false)
					var headerSite HeaderSite
					headerSite, err = readBlockedYamlFile(blockedSitesFilePath)
					if err != nil {
						fmt.Printf("Error reading blocked sites: %v\n", err)
					}
					for _, site := range headerSite.Sites {
						updateExpiryTime(blockedSitesFilePath, site.URL, finalEndTime, false)
						editblockedStatusOnYamlFile(blockedSitesFilePath, site.URL, true)
					}
					displayStatus(blockedSitesFilePath)
					return
				}
			}
			fmt.Println("Not time to block sites")
		}
	}
	fmt.Printf("Schedule %s not found\n", name)
}

// Function to add new goroutine when editing or adding to yaml file
func addNewGoroutine(url string, expiryTime time.Time, isInBackground bool) {
	ctx, cancel := context.WithCancel(context.Background()) // Create a new context for each site
	mu.Lock()
	goroutineContexts[url] = cancel // Use the site URL as the key
	mu.Unlock()

	if isInBackground {
		wg.Add(1)
	}
	go func(expiry time.Time, url string, ctx context.Context) {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done(): // Check if context is cancelled through the cancel() function in cleanup
				fmt.Printf("Goroutine for %s cancelled.\n", url)
				showMenu()
				if isInBackground {
					wg.Done()
				}
				return
			case <-ticker.C: // Counter to automatically remove site after expiry time
				if time.Now().After(expiry) {
					cleanup(false, url) // Replace with actual cleanup logic
					fmt.Printf("Unblocked %s\n", url)
					showMenu()
					if isInBackground {
						wg.Done()
					}
					return
				}
			}
		}
	}(expiryTime, url, ctx)
}

// Function to remove goroutine for a specific site
func removeGouroutine(url string) {
	mu.Lock()
	wgRemove.Add(1)
	if cancel, exists := goroutineContexts[url]; exists { //accessing the goroutine map to find the correct cancel() function for the url
		cancel() // Cancelling the goroutine using the cancel function found in the map
		delete(goroutineContexts, url)
		fmt.Printf("\nCancelled goroutine for site: %s\n", url)
	}
	wgRemove.Done()
	mu.Unlock()
	wgRemove.Wait()
}

// Function to block sites when being run in the background and during startup
func backgroundBlocker(startup bool) {
	fmt.Println("\n**********Background blocking**********")
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
		path = blockedSitesFilePath
	}
	sites, err := readBlockedYamlFile(path)
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		return
	}
	// Go through all sites and block them if they are currently blocked and the duration has not expired
	for _, site := range sites.Sites {
		parsedTime, err := time.Parse(DateTimeLayout, site.Duration)
		if err != nil {
			fmt.Printf("Error parsing duration for site %s: %v\n", site.URL, err)
			continue
		}
		if site.CurrentlyBlocked && time.Now().Before(parsedTime) {
			fmt.Printf("Blocking %s until %s\n", site.Name, site.Duration)
			durationTime, err := time.Parse(DateTimeLayout, site.Duration)
			if err != nil {
				fmt.Printf("Error parsing duration for site %s: %v\n", site.URL, err)
				continue
			}
			blockSites(false, path, site.URL, durationTime, true)
		}
	}
	fmt.Println(goroutineContexts)
	wg.Wait()
	// Once all goroutines are done, cleanup all sites
	cleanup(true, "")
	fmt.Println("Background blocking completed")
}

// Function to check and remove any existing background runtime of application by checking pid on lockfile
func checkAndCleanupExistingInstance() error {
	if _, err := os.Stat(lockFilePath); err == nil {
		// Lock file exists, read the PID
		pidData, err := os.ReadFile(lockFilePath)
		if err != nil {
			return fmt.Errorf("error reading lock file: %v", err)
		}

		if len(pidData) == 0 {
			// Lock file is empty, remove it
			if err := os.Remove(lockFilePath); err != nil {
				return fmt.Errorf("error removing empty lock file: %v", err)
			}
			return nil
		}

		var pid int
		if _, err := fmt.Sscanf(string(pidData), "%d", &pid); err != nil {
			return fmt.Errorf("error parsing PID from lock file: %v", err)
		}

		// Check if the process exists
		if err := syscall.Kill(pid, 0); err != nil {
			if err == syscall.ESRCH {
				// Process does not exist, remove the lock file
				if err := os.Remove(lockFilePath); err != nil {
					return fmt.Errorf("error removing lock file: %v", err)
				}
				return nil
			}
			return fmt.Errorf("error checking process %d: %v", pid, err)
		}

		// Use sudo to send SIGTERM to the existing process
		cmd := exec.Command("sudo", "kill", "-SIGTERM", fmt.Sprintf("%d", pid))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error sending SIGTERM to process %d: %v", pid, err)
		}

		// Wait for the process to exit
		for {
			if err := syscall.Kill(pid, 0); err != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Remove the lock file
		if err := os.Remove(lockFilePath); err != nil {
			return fmt.Errorf("error removing lock file: %v", err)
		}
	}

	return nil
}

func main() {
	// Check if running in background
	if os.Getenv("SELFCONTROL_BACKGROUND") == "1" {
		backgroundBlocker(false)
		return
	}
	if os.Getenv("SELFCONTROL_STARTUP") == "1" {
		backgroundBlocker(true)
		return
	}
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// function to check if user exitted the programme
	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal: %v\n", sig)
		// Clean up all blocked sites
		if err := cleanup(true, ""); err != nil {
			fmt.Printf("Error during cleanup: %v\n", err)
		}
		os.Exit(0)
	}()

	reader := bufio.NewReader(os.Stdin)

	// Verify password before allowing access
	for {
		if !verifyPassword(reader) {
			fmt.Println("Access denied")
		} else {
			break
		}
	}

	for {
		wgRemove.Wait()
		showMenu()
		choice := readUserInput(reader)
		switch choice {
		case "1":
			fmt.Println("Chosen to block sites")

			sitesFileLocation := blockedSitesFilePath
			duration := getDuration(reader)

			// Calculate expiry time
			expiryTime := time.Now().Add(duration)

			headerSites, err := readBlockedYamlFile(sitesFileLocation)
			if err != nil {
				fmt.Printf("Error reading YAML file: %v\n", err)
				continue
			}
			for _, site := range headerSites.Sites {
				updateExpiryTime(sitesFileLocation, site.URL, expiryTime, false)
				fmt.Printf("%s blocked until %s\n", site.Name, expiryTime.Format(DateTimeLayout))
			}

			// Block sites
			if err := blockSites(true, sitesFileLocation, "", expiryTime, false); err != nil {
				fmt.Printf("Error blocking sites: %v\n", err)
				continue
			}

		case "2": // Show current blocked sites
			fmt.Println("Chosen to show current status")
			displayStatus(blockedSitesFilePath)

		case "3": // Add new site to block
			fmt.Print("Enter site URL: ")
			site := FormatString(readUserInput(reader))
			fmt.Print("Enter blocking duration: ")
			duration := readUserInput(reader)
			parsedDuration, err := time.ParseDuration(duration)
			if err != nil {
				fmt.Printf("Invalid duration format: %v\n", err)
				continue
			}
			expiryTime := time.Now().Add(parsedDuration)
			name := GetNameFromURL(site)
			formattedExpiryTime := expiryTime.Format(DateTimeLayout)
			fmt.Print("Expiry Time: ", formattedExpiryTime)
			writeToYamlFile(blockedSitesFilePath, name, site, formattedExpiryTime)
			blockSites(false, blockedSitesFilePath, site, expiryTime, false)

		case "4": // Edit blocked site duration
			fmt.Print("Enter which site to change expiry time: ")
			site := FormatString(readUserInput(reader))
			fmt.Print("Enter new expiry time: ")
			newExpiryTime := time.Now().Add(getDuration(reader))
			if err := updateExpiryTime(blockedSitesFilePath, site, newExpiryTime, true); err != nil {
				fmt.Printf("Error updating expiry time: %v\n", err)
			}

		case "5": // Delete site from yaml configuration
			fmt.Print("Enter site to delete from Config: ")
			site := FormatString(readUserInput(reader))
			cleanup(false, site)
			if err := deleteSiteFromYamlFile(blockedSitesFilePath, "", site); err != nil {
				fmt.Printf("Error deleting site: %v\n", err)
			}
		case "6": // Show schedules
			showSchedules(schedulesFilePath)
		case "7": // Load schedule
			currentTime := time.Now()
			headerSchedule, err := readScheduleYamlFile(schedulesFilePath)
			if err != nil {
				fmt.Printf("Error reading schedule file: %v\n", err)
				continue
			}
			fmt.Print("Enter name of schedule: ")
			name := FormatString(readUserInput(reader))
			loadSchedule(headerSchedule, name, currentTime)

		case "8": // Create new Schedule
			createNewSchedule(reader)

		case "9": // Delete schedule
			fmt.Print("Enter name of schedule to delete: ")
			name := FormatString(readUserInput(reader))
			if err := deleteScheduleFromYamlFile(schedulesFilePath, name); err != nil {
				fmt.Printf("Error deleting schedule: %v\n", err)
			}

		case "10": // Edit schedule
			if err := editSchedulesonYamlFile(schedulesFilePath, reader); err != nil {
				fmt.Printf("Error editing schedule: %v\n", err)
				continue
			}

		case "11": // Change password
			if err := changePassword(reader); err != nil {
				fmt.Printf("Error changing password: %v\n", err)
			} else {
				fmt.Println("Password changed successfully")
			}
		case "12": // Unblock all sites
			fmt.Println("Unblocked all sites")
			cleanup(true, "")
		case "13": // Unblock specific site
			fmt.Print("Enter site to unblock: ")
			site := FormatString(readUserInput(reader))
			if err := cleanup(false, site); err != nil {
				fmt.Printf("Error unblocking site: %v\n", err)
				continue
			}
			fmt.Println("Unblocked site: ", site)
		case "14": // Exit Gracefully
			fmt.Println("Goodbye!")
			cleanup(true, "")
			wgRemove.Wait()
			return
		case "15": // Start process in background
			startBackground()
			return
		default:
			fmt.Println("Invalid option")
		}
	}
}

// Function to start the application in the background while in main function
func startBackground() {

	// Truncate nohup.out file
	_, err := os.Create("nohup.out")
	if err != nil {
		fmt.Println("Error truncating nohup.out file:", err)
		return
	}

	// Get the path to the executable currently running
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting executable path:", err)
		return
	}

	// Check and clean up any existing instance
	if err := checkAndCleanupExistingInstance(); err != nil {
		fmt.Printf("Error checking and cleaning up existing instance: %v\n", err)
		return
	}

	// Create or open nohup.out file
	outFile, err := os.OpenFile("nohup.out", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		return
	}
	defer outFile.Close()

	// Run the same executable in the background using nohup
	cmd := exec.Command("nohup", exe)
	cmd.Env = append(os.Environ(), "SELFCONTROL_BACKGROUND=1")
	cmd.Stdout = outFile
	cmd.Stderr = outFile
	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting special background process:", err)
		return
	}
	// Write the PID of the background process to the lock file
	if err := os.WriteFile(lockFilePath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		fmt.Printf("Error creating lock file: %v\n", err)
		return
	}
	fmt.Println("Selfcontrol application started in background")
}
