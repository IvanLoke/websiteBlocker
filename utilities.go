package main

import (
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
