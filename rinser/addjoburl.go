package rinser

type AddJobURL struct {
	URL           string `json:"url" example:"https://getsamplefiles.com/download/pdf/sample-1.pdf"`
	Lang          string `json:"lang" example:"auto"`
	MaxSizeMB     int    `json:"maxsizemb" example:"2048"`
	MaxTimeSec    int    `json:"maxtimesec" example:"86400"`
	TimeoutSec    int    `json:"timeoutsec" example:"60"`
	CleanupSec    int    `json:"cleanupsec" example:"86400"`
	CleanupGotten bool   `json:"cleanupgotten" example:"true"`
	Private       bool   `json:"private" example:"false"`
}
