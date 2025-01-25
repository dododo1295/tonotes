package test

import (
	"context"
	"main/model"
	"main/repository"
	"main/utils"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name        string
		userAgent   string
		wantBrowser string
		wantOS      string
		wantDevice  string
	}{
		{
			name:        "Chrome on Windows",
			userAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36",
			wantBrowser: "Chrome",
			wantOS:      "Windows",
			wantDevice:  "Desktop",
		},
		{
			name:        "Safari on iPhone",
			userAgent:   "Mozilla/5.0 (iPhone; CPU iPhone OS 14_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Mobile/15E148 Safari/604.1",
			wantBrowser: "Safari",
			wantOS:      "iOS",
			wantDevice:  "iPhone",
		},
		{
			name:        "Empty User Agent",
			userAgent:   "",
			wantBrowser: "Unknown Browser",
			wantOS:      "Unknown OS",
			wantDevice:  "Desktop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browser, os, device := utils.ParseUserAgent(tt.userAgent)

			// Add debug output
			t.Logf("Test case: %s", tt.name)
			t.Logf("UserAgent: %s", tt.userAgent)
			t.Logf("Got browser: %q, want: %q", browser, tt.wantBrowser)
			t.Logf("Got OS: %q, want: %q", os, tt.wantOS)
			t.Logf("Got device: %q, want: %q", device, tt.wantDevice)

			if browser != tt.wantBrowser {
				t.Errorf("ParseUserAgent() browser = %q, want %q", browser, tt.wantBrowser)
			}
			if os != tt.wantOS {
				t.Errorf("ParseUserAgent() os = %q, want %q", os, tt.wantOS)
			}
			if device != tt.wantDevice {
				t.Errorf("ParseUserAgent() device = %q, want %q", device, tt.wantDevice)
			}
		})
	}
}

func TestGenerateSessionName(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		location  string
		want      string
	}{
		{
			name:      "Desktop session with location",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/92.0.4515.131",
			location:  "New York, US",
			want:      "Chrome on Windows (New York, US)",
		},
		{
			name:      "Mobile session with location",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_4 like Mac OS X) Safari/605.1.15",
			location:  "London, UK",
			want:      "Safari on iOS (London, UK)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.GenerateSessionName(tt.userAgent, tt.location)

			// Add debug output
			t.Logf("Test case: %s", tt.name)
			t.Logf("UserAgent: %s", tt.userAgent)
			t.Logf("Location: %s", tt.location)
			t.Logf("Got session name: %q", got)
			t.Logf("Want session name: %q", tt.want)

			// Also get the parsed components for debugging
			browser, os, device := utils.ParseUserAgent(tt.userAgent)
			t.Logf("Parsed components - Browser: %q, OS: %q, Device: %q", browser, os, device)

			if got != tt.want {
				t.Errorf("GenerateSessionName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocationParsing(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		want    string
		wantErr bool
	}{
		{
			name:    "Local IP",
			ip:      "127.0.0.1",
			want:    "Local Network",
			wantErr: false,
		},
		{
			name:    "Internal IP",
			ip:      "192.168.1.1",
			want:    "Local Network",
			wantErr: false,
		},
		{
			name:    "Invalid IP",
			ip:      "invalid-ip",
			want:    "Unknown Location",
			wantErr: false, // Updated to match implementation
		},
		{
			name:    "Empty IP",
			ip:      "",
			want:    "Unknown Location",
			wantErr: false, // Updated to match implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.GetLocationFromIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLocationFromIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetLocationFromIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionNaming(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	sessionRepo := repository.GetSessionRepo(client)

	tests := []struct {
		name      string
		userAgent string
		ipAddress string
		wantName  string
		protected bool
		wantErr   bool
	}{
		{
			name:      "Desktop Chrome Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36",
			ipAddress: "127.0.0.1",
			wantName:  "Chrome on Windows (Local Network)",
			protected: false,
			wantErr:   false,
		},
		{
			name:      "Mobile Safari iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Mobile/15E148 Safari/604.1",
			ipAddress: "192.168.1.1",
			wantName:  "Safari on iOS (Local Network)",
			protected: false,
			wantErr:   false,
		},
		{
			name:      "Protected Desktop Session",
			userAgent: "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36",
			ipAddress: "127.0.0.1",
			wantName:  "Chrome on Windows (Local Network)",
			protected: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create session
			session := &model.Session{
				SessionID:  uuid.New().String(),
				UserID:     "test-user",
				CreatedAt:  time.Now(),
				ExpiresAt:  time.Now().Add(24 * time.Hour),
				DeviceInfo: tt.userAgent,
				IPAddress:  tt.ipAddress,
				Protected:  tt.protected,
			}

			// Generate display name
			location, err := utils.GetLocationFromIP(tt.ipAddress)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLocationFromIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			session.DisplayName = utils.GenerateSessionName(tt.userAgent, location)

			// Store session
			err = sessionRepo.CreateSession(session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Retrieve and verify
			stored, err := sessionRepo.GetSession(session.SessionID)
			if err != nil {
				t.Fatalf("Failed to retrieve session: %v", err)
			}

			if stored.DisplayName != tt.wantName {
				t.Errorf("Session name = %v, want %v", stored.DisplayName, tt.wantName)
			}

			if stored.Protected != tt.protected {
				t.Errorf("Session protected = %v, want %v", stored.Protected, tt.protected)
			}

			// Try to end protected session
			if tt.protected {
				err = sessionRepo.DeleteSession(session.SessionID)
				if err == nil {
					t.Error("Expected error when trying to delete protected session")
				}
			}
		})
	}
}
func TestSessionDeletion(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	sessionRepo := repository.GetSessionRepo(client)

	tests := []struct {
		name      string
		protected bool
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "Delete Unprotected Session",
			protected: false,
			wantErr:   false,
			errMsg:    "",
		},
		{
			name:      "Cannot Delete Protected Session",
			protected: true,
			wantErr:   true,
			errMsg:    "cannot delete protected session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test session
			session := &model.Session{
				SessionID:      uuid.New().String(),
				UserID:         "test-user",
				CreatedAt:      time.Now(),
				ExpiresAt:      time.Now().Add(24 * time.Hour),
				DeviceInfo:     "test-device",
				IPAddress:      "127.0.0.1",
				Protected:      tt.protected,
				DisplayName:    "Test Session",
				LastActivityAt: time.Now(),
				IsActive:       true,
			}

			// Create session
			err := sessionRepo.CreateSession(session)
			if err != nil {
				t.Fatalf("Failed to create test session: %v", err)
			}

			// Try to delete the session
			err = sessionRepo.DeleteSession(session.SessionID)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify session was actually deleted
				deleted, err := sessionRepo.GetSession(session.SessionID)
				if err != nil {
					t.Errorf("Error checking deleted session: %v", err)
				}
				if deleted != nil {
					t.Error("Session still exists after deletion")
				}
			}
		})
	}

	// Test deleting non-existent session
	t.Run("Delete Non-existent Session", func(t *testing.T) {
		err := sessionRepo.DeleteSession("non-existent-id")
		if err == nil {
			t.Error("Expected error when deleting non-existent session")
		}
		if err.Error() != "session not found" {
			t.Errorf("Expected 'session not found' error, got %q", err.Error())
		}
	})

	// Test deleting with empty session ID
	t.Run("Delete With Empty SessionID", func(t *testing.T) {
		err := sessionRepo.DeleteSession("")
		if err == nil {
			t.Error("Expected error when deleting with empty session ID")
		}
		if err.Error() != "sessionID cannot be empty" {
			t.Errorf("Expected 'sessionID cannot be empty' error, got %q", err.Error())
		}
	})
}
