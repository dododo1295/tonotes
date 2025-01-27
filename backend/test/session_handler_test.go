package test

import (
	"encoding/json"
	"main/handler"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestSessionHandler(t *testing.T) {
	// Setup
	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	// Set the global MongoDB client for utils package
	utils.MongoClient = client

	// Initialize session repository
	sessionRepo := repository.GetSessionRepo(client)

	gin.SetMode(gin.TestMode)

	t.Run("GetActiveSessions", func(t *testing.T) {
		// Setup
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create test session
		testSession := &model.Session{
			SessionID:      uuid.New().String(),
			UserID:         "test-user",
			DisplayName:    "Test Session",
			DeviceInfo:     "Test Device",
			IPAddress:      "127.0.0.1",
			Location:       "Test Location",
			CreatedAt:      time.Now(),
			LastActivityAt: time.Now(),
			ExpiresAt:      time.Now().Add(24 * time.Hour),
			IsActive:       true,
		}
		if err := sessionRepo.CreateSession(testSession); err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Set user context
		c.Set("user_id", "test-user")
		c.Set("session_id", testSession.SessionID)

		// Test
		handler.GetActiveSessions(c, sessionRepo)

		// Assertions
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		var response utils.Response
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		data, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Response data is not in expected format")
		}

		sessions, ok := data["sessions"].([]interface{})
		if !ok {
			t.Fatal("Sessions data is not in expected format")
		}

		if len(sessions) != 1 {
			t.Errorf("Expected 1 session, got %d", len(sessions))
		}
	})

	t.Run("LogoutAllSessions", func(t *testing.T) {
		// Setup
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create multiple test sessions
		for i := 0; i < 3; i++ {
			testSession := &model.Session{
				SessionID:   uuid.New().String(),
				UserID:      "test-user",
				DisplayName: "Test Session",
				DeviceInfo:  "Test Device",
				IsActive:    true,
			}
			if err := sessionRepo.CreateSession(testSession); err != nil {
				t.Fatalf("Failed to create test session: %v", err)
			}
		}

		// Set user context
		c.Set("user_id", "test-user")

		// Test
		handler.LogoutAllSessions(c, sessionRepo)

		// Assertions
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Verify all sessions are ended
		sessions, err := sessionRepo.GetUserActiveSessions("test-user")
		if err != nil {
			t.Fatalf("Failed to get active sessions: %v", err)
		}
		if len(sessions) != 0 {
			t.Errorf("Expected 0 active sessions, got %d", len(sessions))
		}
	})

	t.Run("LogoutSession", func(t *testing.T) {
		// Setup
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create test session
		testSession := &model.Session{
			SessionID:   uuid.New().String(),
			UserID:      "test-user",
			DisplayName: "Test Session",
			DeviceInfo:  "Test Device",
			IsActive:    true,
		}
		if err := sessionRepo.CreateSession(testSession); err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Set user context and params
		c.Set("user_id", "test-user")
		c.Params = append(c.Params, gin.Param{Key: "session_id", Value: testSession.SessionID})

		// Test
		handler.LogoutSession(c, sessionRepo)

		// Assertions
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Verify session is deleted
		session, err := sessionRepo.GetSession(testSession.SessionID)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		if session != nil {
			t.Error("Session should have been deleted")
		}
	})

	t.Run("CreateSession", func(t *testing.T) {
		// Setup
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Mock request context
		c.Request = httptest.NewRequest("POST", "/", nil)
		c.Request.Header.Set("User-Agent", "Mozilla/5.0 (Test)")

		// Test
		err := handler.CreateSession(c, "test-user", sessionRepo)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Get cookie
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_id" {
				sessionCookie = cookie
				break
			}
		}

		// Assertions
		if sessionCookie == nil {
			t.Error("Session cookie not found")
		}

		// Verify session was created
		session, err := sessionRepo.GetSession(sessionCookie.Value)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		if session == nil {
			t.Error("Session not found in database")
		}
		if session.UserID != "test-user" {
			t.Errorf("Expected user ID %s, got %s", "test-user", session.UserID)
		}
	})

	t.Run("GetSessionDetails", func(t *testing.T) {
		// Setup
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create test session
		testSession := &model.Session{
			SessionID:   uuid.New().String(),
			UserID:      "test-user",
			DisplayName: "Test Session",
			DeviceInfo:  "Test Device",
			IsActive:    true,
		}
		if err := sessionRepo.CreateSession(testSession); err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Set user context and params
		c.Set("user_id", "test-user")
		c.Params = append(c.Params, gin.Param{Key: "session_id", Value: testSession.SessionID})

		// Test
		handler.GetSessionDetails(c, sessionRepo)

		// Assertions
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		var response utils.Response
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		data, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Response data is not in expected format")
		}
		if data["session"] == nil {
			t.Error("Session details not found in response")
		}
	})
}
