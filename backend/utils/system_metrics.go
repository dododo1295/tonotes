package utils

import (
	"log"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

// GetCPUUsage returns the current CPU usage as a percentage
func GetCPUUsage() float64 {
	percentage, err := cpu.Percent(time.Second, false)
	if err != nil {
		log.Printf("Error getting CPU usage: %v", err)
		return 0
	}
	if len(percentage) > 0 {
		return percentage[0]
	}
	return 0
}
