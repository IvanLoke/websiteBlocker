package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Functions for blocked-sites.yaml

// Function to read blocked yaml file, returns a HeaderSite struct
func readBlockedYamlFile(filename string) (HeaderSite, error) {
	file, err := os.Open(filename)
	if err != nil {
		return HeaderSite{}, err
	}
	defer file.Close()

	var headerSites HeaderSite
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&headerSites)
	if err != nil {
		return HeaderSite{}, err
	}
	return headerSites, nil
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

// Functions for schedules.yaml

// Function to read schedule yaml file, returns a HeaderSchedule struct
func readScheduleYamlFile(filename string) (HeaderSchedule, error) {
	file, err := os.Open(filename)
	if err != nil {
		return HeaderSchedule{}, err
	}
	defer file.Close()

	var headerSchedule HeaderSchedule
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&headerSchedule)
	if err != nil {
		return HeaderSchedule{}, err
	}
	return headerSchedule, nil
}

func createNewSchedule(reader *bufio.Reader) {
	fmt.Print("Enter name of schedule: ")
	name := readUserInput(reader)
	fmt.Print("Enter days to block seperated by commas: ")
	days := strings.TrimSpace(readUserInput(reader))
	startTimeFormatted := queryForTime(reader, true)
	endTimeFormatted := queryForTime(reader, false)
	if err := checkStartBeforeEnd(startTimeFormatted, endTimeFormatted); err != nil {
		fmt.Println("Error in time inputs: ", err)
		return
	}
	var cleanedDays []string
	re := regexp.MustCompile(`\s*,\s*|\s+`)
	splitDays := re.Split(days, -1)

	// Split days string into a slice and trim whitespace
	for _, day := range splitDays {
		trimmedDay := FormatString(day)               // Trim whitespace
		cleanedDays = append(cleanedDays, trimmedDay) // Add to cleaned slice
	}

	if err := checkValidDay(cleanedDays); err != nil {
		fmt.Printf("Error checking valid day: %v\n", err)
		return
	}

	if err := writeToScheduleYamlFile("schedules.yaml", name, cleanedDays, startTimeFormatted, endTimeFormatted); err != nil {
		fmt.Println("Error writing to schedule yaml file: ", err)
	}
}

// Function to create a new schedule and write in to yaml file
func writeToScheduleYamlFile(filename string, name string, days []string, startTime string, endTime string) error {
	headerSchedule, err := readScheduleYamlFile(filename)
	if err != nil {
		return err
	}

	for _, schedule := range headerSchedule.Schedules {
		if schedule.Name == name {
			return fmt.Errorf("Schedule already exists")
		}
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
