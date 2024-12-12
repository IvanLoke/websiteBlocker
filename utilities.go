package main

import (
	"errors"
	"strconv"
	"strings"
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
