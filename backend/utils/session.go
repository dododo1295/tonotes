package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	ua "github.com/mileusna/useragent"
)

type GeoIPResponse struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

// ParseUserAgent extracts useful information from User-Agent string
func ParseUserAgent(userAgent string) (browser, os, device string) {
	if userAgent == "" {
		return "Unknown Browser", "Unknown OS", "Desktop"
	}

	parsedUA := ua.Parse(userAgent)

	// Get browser name (without version)
	if parsedUA.Name != "" {
		browser = parsedUA.Name
	} else {
		browser = "Unknown Browser"
	}

	// Get OS name (without version)
	if parsedUA.OS != "" {
		os = parsedUA.OS
	} else {
		os = "Unknown OS"
	}

	// Determine device type
	device = "Desktop" // Default
	if parsedUA.Mobile {
		if strings.Contains(userAgent, "iPhone") {
			device = "iPhone"
		} else {
			device = "Mobile"
		}
	} else if parsedUA.Tablet {
		device = "Tablet"
	}

	return strings.TrimSpace(browser), strings.TrimSpace(os), device
}

// GetLocationFromIP fetches location information from IP address
func GetLocationFromIP(ip string) (string, error) {
	// Skip for localhost/internal IPs or empty IP
	// Handle empty IP explicitly
	if ip == "" {
		return "Unknown Location", nil
	}

	// Skip for localhost/internal IPs
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") {
		return "Local Network", nil
	}

	if ip == "invalid-ip" {
		return "Unknown Location", nil
	}
	// You can use various IP geolocation services. Here's an example with ipapi.co
	// Consider using a paid service in production for better reliability and rate limits
	url := fmt.Sprintf("https://ipapi.co/%s/json/", ip)
	resp, err := http.Get(url)
	if err != nil {
		return "Unknown Location", nil
	}
	defer resp.Body.Close()

	var geoIP GeoIPResponse
	if err := json.NewDecoder(resp.Body).Decode(&geoIP); err != nil {
		return "Unknown Location", nil
	}

	// Format location string
	location := "Unknown Location"
	if geoIP.City != "" && geoIP.Country != "" {
		location = fmt.Sprintf("%s, %s", geoIP.City, geoIP.Country)
	} else if geoIP.Country != "" {
		location = geoIP.Country
	}

	return location, nil
}

// GenerateSessionName creates a user-friendly session name
func GenerateSessionName(userAgent string, location string) string {
	browser, os, _ := ParseUserAgent(userAgent)

	// Basic format: "Browser on OS"
	name := fmt.Sprintf("%s on %s", browser, os)

	// Add location if provided
	if location != "" {
		name = fmt.Sprintf("%s (%s)", name, location)
	} else {
		name = fmt.Sprintf("%s (%s)", name, "Unknown Location")
	}

	return name
}
