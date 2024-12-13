package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

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

	writeToScheduleYamlFile("schedules.yaml", name, cleanedDays, startTimeFormatted, endTimeFormatted)
}
