package utils

import (
	"os"
	"strconv"
	"time"
)

// getEnvAsInt retrieves an environment variable and converts it to an integer
func GetEnvAsInt(key string, defaultVal int) int {
	if value, exists := os.LookupEnv(key); exists {
		if result, err := strconv.Atoi(value); err == nil {
			return result
		}
	}
	return defaultVal
}

// getEnvAsUint64 retrieves an environment variable and converts it to uint64
func GetEnvAsUint64(key string, defaultVal uint64) uint64 {
	if value, exists := os.LookupEnv(key); exists {
		if result, err := strconv.ParseUint(value, 10, 64); err == nil {
			return result
		}
	}
	return defaultVal
}

// getEnvAsDuration retrieves an environment variable and converts it to Duration
func GetEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if result, err := time.ParseDuration(value); err == nil {
			return result
		}
	}
	return defaultVal
}

// getEnvAsBool retrieves an environment variable and converts it to boolean
func GetEnvAsBool(key string, defaultVal bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if result, err := strconv.ParseBool(value); err == nil {
			return result
		}
	}
	return defaultVal
}

// getEnvAsString retrieves an environment variable or returns a default value
func GetEnvAsString(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
