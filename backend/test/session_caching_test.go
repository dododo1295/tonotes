package test

import (
	"main/model"
	"main/services"
	"testing"
	"time"
)

func TestSessionCache(t *testing.T) {
    // Set up test Redis connection
    cache, err := services.NewSessionCache("redis://localhost:6379/1")
    if err != nil {
        t.Fatalf("Failed to create session cache: %v", err)
    }
    defer cache.Close()

    // Create test session
    session := &model.Session{
        SessionID: "test-session",
        UserID: "test-user",
        DisplayName: "Chrome on Windows (New York, US)",
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    // Test setting session
    t.Run("Set Session", func(t *testing.T) {
        err := cache.SetSession(session)
        if err != nil {
            t.Errorf("SetSession() error = %v", err)
        }
    })

    // Test getting session
    t.Run("Get Session", func(t *testing.T) {
        got, err := cache.GetSession(session.SessionID)
        if err != nil {
            t.Errorf("GetSession() error = %v", err)
            return
        }
        if got.SessionID != session.SessionID {
            t.Errorf("GetSession() = %v, want %v", got.SessionID, session.SessionID)
        }
    })

    // Test session versioning
    t.Run("Session Versioning", func(t *testing.T) {
        err := cache.IncrementSessionVersion(session.UserID)
        if err != nil {
            t.Errorf("IncrementSessionVersion() error = %v", err)
        }

        needsRefresh, err := cache.NeedsRefresh(session.UserID)
        if err != nil {
            t.Errorf("NeedsRefresh() error = %v", err)
        }
        if !needsRefresh {
            t.Error("Expected session to need refresh after version increment")
        }
    })
}
