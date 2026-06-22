package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dorm-repair-system/internal/config"
	"dorm-repair-system/internal/global"
	"dorm-repair-system/pkg/e"
	"dorm-repair-system/pkg/response"
	"dorm-repair-system/pkg/utils"

	casbin "github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	// Set gin to test mode to avoid debugging logs
	gin.SetMode(gin.TestMode)
}

// setupTest sets up mock configuration, logger, and Casbin enforcer for testing.
// It returns a cleanup function to restore original globals.
func setupTest(t *testing.T) func() {
	// Backup original global variables
	oldConfig := global.Config
	oldLogger := global.Logger
	oldEnforcer := global.Enforcer

	// Mock Configuration
	global.Config = &config.Config{
		JWT: config.JWTConfig{
			Secret: "my-super-secret-test-key-32-characters-long",
			Expire: 3600, // 1 hour
		},
	}

	// Mock Logger
	global.Logger = zap.NewNop()

	// Mock Casbin Enforcer using the project's model file
	enforcer, err := casbin.NewEnforcer("../../config/rbac_model.conf")
	if err != nil {
		t.Fatalf("failed to initialize casbin enforcer: %v", err)
	}

	// Add basic test policies
	// Matcher rule: g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act) || r.sub == "Admin"
	_, _ = enforcer.AddPolicy("Student", "/api/v1/workorders", "POST")
	_, _ = enforcer.AddPolicy("Student", "/api/v1/workorders", "GET")
	_, _ = enforcer.AddPolicy("Worker", "/api/v1/workorders/:id/status", "PUT")
	_, _ = enforcer.AddPolicy("Worker", "/api/v1/workorders", "GET")

	global.Enforcer = enforcer

	// Return teardown function
	return func() {
		global.Config = oldConfig
		global.Logger = oldLogger
		global.Enforcer = oldEnforcer
	}
}

// TestJWTAuth tests the JWT authentication middleware.
func TestJWTAuth(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Helper to generate a valid token
	validToken, err := utils.GenerateToken(1, "testuser", "Student")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Setup Gin router with the middleware
	r := gin.New()
	r.Use(JWTAuth())
	r.GET("/test-auth", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		c.JSON(http.StatusOK, gin.H{
			"userID":   userID,
			"username": username,
			"role":     role,
		})
	})

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedCode   e.ErrCode
		expectedMsg    string
		verifyContext  bool
	}{
		{
			name:           "Missing Authorization Header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   e.Unauthorized,
			expectedMsg:    "Authorization header is required",
		},
		{
			name:           "Incorrect Header Format",
			authHeader:     "BearerTokenValue",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   e.Unauthorized,
			expectedMsg:    "Authorization header format must be Bearer {token}",
		},
		{
			name:           "Invalid Token Signature/Value",
			authHeader:     "Bearer invalid.token.value",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   e.Unauthorized,
			expectedMsg:    "Invalid or expired token",
		},
		{
			name:           "Valid Token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			verifyContext:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/test-auth", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !tt.verifyContext {
				// Parse custom fail response
				var resp response.Response
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				if err != nil {
					t.Fatalf("failed to parse response body: %v", err)
				}
				if resp.Code != tt.expectedCode {
					t.Errorf("expected error code %d, got %d", tt.expectedCode, resp.Code)
				}
				if resp.Msg != tt.expectedMsg {
					t.Errorf("expected error message %q, got %q", tt.expectedMsg, resp.Msg)
				}
			} else {
				// Verify context attributes passed successfully to handler
				var contextData map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &contextData)
				if err != nil {
					t.Fatalf("failed to parse success response body: %v", err)
				}
				if contextData["userID"] != float64(1) { // json unmarshals numbers to float64
					t.Errorf("expected userID in context 1, got %v", contextData["userID"])
				}
				if contextData["username"] != "testuser" {
					t.Errorf("expected username in context 'testuser', got %q", contextData["username"])
				}
				if contextData["role"] != "Student" {
					t.Errorf("expected role in context 'Student', got %q", contextData["role"])
				}
			}
		})
	}
}

// TestCasbinRBAC tests the Casbin-based RBAC middleware.
func TestCasbinRBAC(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Setup Gin router
	r := gin.New()
	
	// Middleware to set role dynamically from request header for test flexibility
	r.Use(func(c *gin.Context) {
		role := c.GetHeader("Test-Role")
		if role != "" {
			c.Set("role", role)
		}
		c.Next()
	})
	
	r.Use(CasbinRBAC())
	
	// Add test handlers representing RESTful endpoints matching the policies
	r.POST("/api/v1/workorders", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/api/v1/workorders", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.PUT("/api/v1/workorders/:id/status", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name           string
		role           string // Custom test header
		method         string
		path           string
		expectedStatus int
		expectedCode   e.ErrCode
		expectedMsg    string
	}{
		{
			name:           "No Role In Context",
			role:           "", // Will not set "role" context
			method:         http.MethodPost,
			path:           "/api/v1/workorders",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   e.Unauthorized,
			expectedMsg:    "Unauthorized",
		},
		{
			name:           "Student - Post Work Order (Allowed)",
			role:           "Student",
			method:         http.MethodPost,
			path:           "/api/v1/workorders",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Student - Get Work Orders (Allowed)",
			role:           "Student",
			method:         http.MethodGet,
			path:           "/api/v1/workorders",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Student - Put Status (Denied/Forbidden)",
			role:           "Student",
			method:         http.MethodPut,
			path:           "/api/v1/workorders/123/status",
			expectedStatus: http.StatusForbidden,
			expectedCode:   e.Forbidden,
			expectedMsg:    "Forbidden",
		},
		{
			name:           "Worker - Put Status (Allowed, KeyMatch2 URL param)",
			role:           "Worker",
			method:         http.MethodPut,
			path:           "/api/v1/workorders/123/status",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Worker - Post Work Order (Denied/Forbidden)",
			role:           "Worker",
			method:         http.MethodPost,
			path:           "/api/v1/workorders",
			expectedStatus: http.StatusForbidden,
			expectedCode:   e.Forbidden,
			expectedMsg:    "Forbidden",
		},
		{
			name:           "Admin - Bypasses All Rules (Superuser Allowed)",
			role:           "Admin",
			method:         http.MethodPost,
			path:           "/api/v1/workorders",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Admin - Bypasses Non-Existent Path (Superuser Allowed)",
			role:           "Admin",
			method:         http.MethodPut,
			path:           "/api/v1/workorders/123/status",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			if tt.role != "" {
				req.Header.Set("Test-Role", tt.role)
			}

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus != http.StatusOK {
				var resp response.Response
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				if err != nil {
					t.Fatalf("failed to parse response body: %v", err)
				}
				if resp.Code != tt.expectedCode {
					t.Errorf("expected error code %d, got %d", tt.expectedCode, resp.Code)
				}
				if resp.Msg != tt.expectedMsg {
					t.Errorf("expected error message %q, got %q", tt.expectedMsg, resp.Msg)
				}
			}
		})
	}
}
