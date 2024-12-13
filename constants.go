package main

const (
	// Simplified DateTimeLayout
	DateTimeLayout       = "2006-01-02 15:04:05 -0700"
	hostsFile            = "/etc/hosts"
	blockedSitesFilePath = "configs/blocked-sites.yaml"
	schedulesFilePath    = "configs/schedules.yaml"
)

var daysOfWeek = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
