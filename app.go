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

func showMenu() {
	time.Sleep(200 * time.Millisecond)
	fmt.Println("\n\n **********Self Control Menu**********")
	fmt.Println("0: Block for specific time")
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

	//Verify password before allowing access
	for {
		if !verifyPassword(reader) {
			fmt.Println("Access denied")
		} else {
			break
		}
	}

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
		case "0":
			fmt.Println("Block for specific time")
			duration := getDuration(reader)

			// Calculate the expiry time directly by adding the duration to the current time
			expiryTime := time.Now().Add(duration)

			// Call the blockSitesCustomTime function with the calculated expiry time
			blockSitesCustomTime(configFilePath, false, expiryTime)
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
				if endTime, err := getExpiryTime(); err != nil {
					fmt.Println(err)
				} else {
					printAllSites()
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

// Menu for editing configs
func editConfigMenu() {
	fmt.Println("\n\n **********Edit Schedule Menu**********")
	fmt.Println("1: Add new site to config")
	fmt.Println("2: Delete site from config")
	fmt.Println("3: Show schedule")
	fmt.Println("4: Edit schedule")
	fmt.Println("5: Exit")
	fmt.Print("Choose an option: ")
}

// Switch case logic for editing configs
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

// Menu for editing schedules
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

// Switch case logic for editing schedules
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
