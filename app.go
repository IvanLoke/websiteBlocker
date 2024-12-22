package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
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
	fmt.Println("\n\n **********Self Control Menu**********")
	fmt.Println("1: Block sites using the schedule")
	fmt.Println("2: Show current status")
	fmt.Println("3: Enter strict mode")
	fmt.Println("4: Add sites to block")
	fmt.Println("5: Acess normal mode menu")
	fmt.Println("6: Edit schedules")
	fmt.Println("7: Change password")
	fmt.Println("8: Exit")
	fmt.Print("Choose an option: ")
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

// Removes entries inside etc/hosts that were added by selfcontrol and updates the yaml file
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
				switchModeStrict(2)
				if isInBackground {
					wg.Done()
				}
				return
			case <-ticker.C: // Counter to automatically remove site after expiry time
				if time.Now().After(expiry) {
					cleanupStrict() // Replace with actual cleanup logic
					switchModeStrict(2)
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

		if _, err := getExpiryTime(); err != nil { // if error, means there are no sites left to block
			switchModeStrict(2)
		}
	}
	wgRemove.Done()
	mu.Unlock()
	wgRemove.Wait()
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
	} else {
		return fmt.Errorf("file does not exist")
	}

	return nil
}

func main() {
	// Check if running in background
	if os.Getenv("SELFCONTROL_BACKGROUND") == "1" {
		backgroundBlocker(false)
		changeBlockOnRestartStatus("false")
		if err := os.Remove(lockFilePath); err != nil {
			fmt.Printf("Error removing lock file: %v\n", err)
			return
		}
		return
	}
	if os.Getenv("SELFCONTROL_STARTUP") == "1" {
		if blocking, err := getBlockOnRestartStatus(); err != nil {
			fmt.Println("Error getting block on restart status:", err)
			return
		} else if blocking == "true" {
			backgroundBlocker(true)
			changeBlockOnRestartStatus("false")
			if err := os.Remove(lockFilePath); err != nil {
				fmt.Printf("Error removing lock file: %v\n", err)
				return
			}
		}
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
		if len(goroutineContexts) > 0 {
			startBackground()
			changeBlockOnRestartStatus("true")
		}
		os.Exit(0)
	}()

	reader := bufio.NewReader(os.Stdin)

	// //Verify password before allowing access
	// for {
	// 	if !verifyPassword(reader) {
	// 		fmt.Println("Access denied")
	// 	} else {
	// 		break
	// 	}
	// }

	// Check if service is enabled
	if !checkForServiceFile() {
		fmt.Println("Creating service file")
		err := createServiceFile()
		if err != nil {
			fmt.Println("Error creating service file")
			return
		}
		fmt.Println("Service file created")
		fmt.Println("Updating file path in constants.go")
		initAbsPathToSelfControl()
	}

	// checking if the service was running in the background, if it was, block all sites that should be getting blocked based on schedule
	err := checkAndCleanupExistingInstance() //error indicates that no need to continue blocking sites
	blocking, _ := getBlockOnRestartStatus()
	if err == nil && blocking == "true" {
		fmt.Println("Background instance detected, continuing...")
		if status, err := getBlockCustomTimeStatus(); err == nil && status == "true" {
			if endTime, err := getEndingTime(); err != nil {
				fmt.Println("Error getting ending time:", err)
			} else {
				parsedEndTime, err := time.Parse(DateTimeLayout, endTime)
				if err != nil {
					fmt.Println("Error parsing end time:", err)
					return
				}
				blockSitesCustomTime(configFilePath, true, parsedEndTime)
			}
		} else {
			blockSitesStrict(configFilePath, true)
		}
	} else {
		fmt.Printf("No background instance detected: %s \n", err)
	}
	changeBlockOnRestartStatus("false")
	if len(goroutineContexts) == 0 {
		switchModeStrict(2)
	}
	// // Main menu loop
	for {
		wgRemove.Wait()
		time.Sleep(200 * time.Millisecond)
		showMenu()
		choice := readUserInput(reader)
		switch choice {
		case "map":
			fmt.Println(goroutineContexts)
			fmt.Println("Number of goroutines: ", len(goroutineContexts))
		case "q":
			fmt.Println("Block for specific time")
			duration := getDuration(reader)

			// Calculate the expiry time directly by adding the duration to the current time
			expiryTime := time.Now().Add(duration)

			// Call the blockSitesCustomTime function with the calculated expiry time
			blockSitesCustomTime(configFilePath, false, expiryTime)
		case "w":
			cleanupStrict()
		case "1":
			if mode, err := checkMode(); err == nil && mode == "strict" && len(goroutineContexts) > 0 {
				fmt.Println("Wait until strict mode is done before starting schedule")
			} else {
				fmt.Println("Block using schedule")
				blockSitesStrict(configFilePath, false)
			}
		case "2":
			fmt.Println("Show current status")
			if len(goroutineContexts) == 0 {
				fmt.Println("No sites are currently blocked")
				continue
			}
			var configs *Config
			if configs, err = readConfig(configFilePath); err != nil {
				fmt.Printf("Error reading config file: %v\n", err)
				return
			}

			// Check if blocking is active
			if status, err := getBlockCustomTimeStatus(); err != nil {
				fmt.Printf("Error getting block status: %v\n", err)
			} else if status == "true" {
				// If blocking is active, get the ending time
				if endTime, err := getEndingTime(); err != nil {
					fmt.Println("Error getting ending time:", err)
				} else {
					fmt.Printf("Block is in effect until %s\n", endTime)
					printAllSites()
				}
			} else {
				// If blocking is not active, print the sites to block
				if endTime, err := printSitesToBlock(configs, true); err != nil {
					fmt.Println(err)
				} else {
					fmt.Printf("\nBlock is in effect until %s\n", endTime)
				}
			}

		case "3":
			switchingStrictModeMenu(reader)
		case "4":
			fmt.Print("Enter new site to block: ")
			site := FormatString(readUserInput(reader))
			addNewSiteToConfig(site)
			fmt.Println("Do you want to start blocking all new sites now?")
			fmt.Println("1: Yes")
			fmt.Println("2: No")
			fmt.Print("Enter choice: ")
			choice := readUserInput(reader)
			if choice == "1" {
				// Check if blocking is active
				status, err := getBlockCustomTimeStatus()
				if err != nil {
					fmt.Printf("Error getting block status: %v\n", err)
					return // Exit or handle the error as needed
				}

				if status == "true" {
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
					// Block sites using the custom time
					if err := blockSitesCustomTime(configFilePath, false, parsedEndTime); err != nil {
						fmt.Printf("Error blocking sites: %v\n", err)
						return
					}
				} else {
					cleanupStrict()
					// If blocking is not active, block all sites in strict mode
					if err := blockSitesStrict(configFilePath, false); err != nil {
						fmt.Printf("Error blocking sites in strict mode: %v\n", err)
						return
					}
				}

				// Check and switch mode if necessary
				mode, err := checkMode()
				if err != nil {
					fmt.Printf("Error checking mode: %v\n", err)
					return
				}
				if mode == "strict" {
					time.Sleep(202 * time.Millisecond)
					switchModeStrict(1) // Switch to strict mode
				}
			}
		case "5":
			if accessingMenusInStrictMode() {
				normalModeMenuSelection(reader)
			}

		case "6":
			if accessingMenusInStrictMode() {
				editConfigSelection(reader)
			}
		case "7": // Change password
			if err := changePassword(reader); err != nil {
				fmt.Printf("Error changing password: %v\n", err)
			} else {
				fmt.Println("Password changed successfully")
			}
		case "8": // Start process in background
			if len(goroutineContexts) > 0 {
				startBackground()
				changeBlockOnRestartStatus("true")
			}
			return

		default:
			fmt.Println("Invalid option")
		}
	}
}

func normalModeMenuSelection(reader *bufio.Reader) {
	fmt.Println("Accessed normal mode menu")
normalModeLoop:
	for {
		normalModeMenu()
		normalmodechoice := readUserInput(reader)

		switch normalmodechoice {
		case "1":
			fmt.Print("Enter new site to block: ")
			site := FormatString(readUserInput(reader))
			addNewSiteToConfig(site)

			// Reapply the blocking rules after adding the new site
			cleanupStrict()
			blockSitesStrict(configFilePath, false)

		case "2":
			fmt.Println("Stop blocking all sites")
			cleanupStrict()

		case "3":
			deleteAndUnblockSiteFromConfig(reader)

		case "4":
			fmt.Println("\nExiting...")
			break normalModeLoop

		}
	}
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
	if err := checkAndCleanupExistingInstance(); err != nil && err.Error() != "file does not exist" {
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
	fmt.Println("Selfcontrol started in background")
}

func editConfigMenu() {
	fmt.Println("\n\n **********Edit Schedule Menu**********")
	fmt.Println("1: Add new site to config")
	fmt.Println("2: Delete site from config")
	fmt.Println("3: Show schedule")
	fmt.Println("4: Edit schedule")
	fmt.Println("5: Exit")
	fmt.Print("Choose an option: ")
}

func editConfigSelection(reader *bufio.Reader) {
editScheduleLoop:
	for {
		editConfigMenu()
		choice := readUserInput(reader)
		switch choice {
		case "1":
			fmt.Print("Enter new site to block: ")
			site := FormatString(readUserInput(reader))
			addNewSiteToConfig(site)
		case "2":
			deleteSiteFromConfig(reader)
		case "3":
			displaySchedule()
		case "4":
			editScheduleSelection(reader)

		case "5":
			fmt.Println("\nExiting...")
			break editScheduleLoop

		default:
			fmt.Println("Invalid option")
		}
	}
}

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

func editScheduleMenu() {
	fmt.Println("\n\n **********Edit Schedule Menu**********")
	fmt.Println("1: Add new site to schedule")
	fmt.Println("2: Delete site from schedule")
	fmt.Println("3: Add day to schedule")
	fmt.Println("4: Delete day from schedule")
	fmt.Println("5: Add time range for day in schedule")
	fmt.Println("6: Delete time range for day in schedule")
	fmt.Println("7: Edit start time of day in schedule")
	fmt.Println("8: Edit end time of day in schedule")
	fmt.Println("9: Exit")
	fmt.Printf("Choose an option: ")
}

func editScheduleSelection(reader *bufio.Reader) {
editScheduleLoop:
	for {
		editScheduleMenu()
		choice := readUserInput(reader)
		switch choice {
		case "1":
			fmt.Print("Enter new site to block: ")
			site := FormatString(readUserInput(reader))
			addNewSiteToConfig(site)
		case "2":
			deleteSiteFromConfig(reader)
		case "3":
			addDayToSchedule(reader)
		case "4":
			deleteDayFromSchedule(reader)
		case "5":
			addTimeRangeForDay(reader)
		case "6":
			deleteTimeRangeForDay(reader)
		case "7":
			editTimeforSchedule(true, reader)
		case "8":
			editTimeforSchedule(false, reader)
		case "9":
			fmt.Println("\nExiting...")
			break editScheduleLoop

		default:
			fmt.Println("Invalid option")
		}
	}
}

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
