package main

const (
	DateTimeLayout            = "2006-01-02 15:04:05 -0700"
	hostsFile                 = "/etc/hosts"
	blockedSitesFilePathRoot  = "/home/ivan/work/voyager/selfcontrol/configs/blocked-sites.yaml"
	blockedSitesFilePath      = "configs/blocked-sites.yaml"
	schedulesFilePath         = "configs/schedules.yaml"
	passwordFilePath          = "configs/.password"
	lockFilePath              = "tmp/selfcontrol.lock"
	absolutePathToSelfControl = "/home/ivan/work/voyager/selfcontrol" //update this to your path to selfcontrol app
)

var daysOfWeek = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
