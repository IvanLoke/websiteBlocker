package main

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var prefixes = [3]string{"www.", "https://", "http://"}
var suffixes = [2]string{".com", ".org"}

func FormatString(data string) string {
	return strings.TrimSpace(strings.ToLower(data))
}

// Function to extract name from url
func GetNameFromURL(url string) string {
	for _, prefix := range prefixes {
		url = strings.TrimPrefix(url, prefix)
	}
	for _, suffix := range suffixes {
		url = strings.TrimSuffix(url, suffix)
	}
	return FormatString(url)
}

// Function to format time in "HH:MM" format
func FormatTime(time string) (string, error) {
	if len(time) > 5 || len(time) < 4 || (len(time) == 5 && time[2] != ':') {
		return "", errors.New("invalid time format")
	}
	if _, err := strconv.Atoi(strings.Replace(time, ":", "", -1)); err != nil {
		return "", errors.New("invalid time format")
	}
	if len(time) == 4 {
		return (time[:2] + ":" + time[2:]), nil
	} else {
		return time, nil
	}
}

// Function to check if start time is before end time
func checkStartBeforeEnd(startTime string, endTime string) error {
	formattedStartTime, errStart := time.Parse("15:04", startTime)
	formatteedEndTime, errEnd := time.Parse("15:04", endTime)
	if errStart != nil {
		fmt.Println("Error parsing start time: ", errStart)
		return errStart
	}
	if errEnd != nil {
		fmt.Println("Error parsing end time: ", errEnd)
		return errEnd
	}
	if formatteedEndTime.Before(formattedStartTime) {
		return errors.New("end time cannot be before start time")
	}
	return nil
}

// Function to check if day is valid
func checkValidDay(days []string) error {
	for _, day := range days {
		valid := false
		for _, validDay := range daysOfWeek {
			if strings.EqualFold(day, validDay) {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid day entered: %s", day)
		}
	}
	return nil
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
