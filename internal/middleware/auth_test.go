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
	// 将 gin 设置为测试模式以避免调试日志输出
	gin.SetMode(gin.TestMode)
}

// setupTest 初始化用于测试的配置、日志和 Casbin 权限管理器
// 返回一个清理函数以便在测试结束后恢复全局变量
func setupTest(t *testing.T) func() {
	// 备份原全局变量
	oldConfig := global.Config
	oldLogger := global.Logger
	oldEnforcer := global.Enforcer

	// 模拟配置数据
	global.Config = &config.Config{
		JWT: config.JWTConfig{
			Secret: "my-super-secret-test-key-32-characters-long",
			Expire: 3600, // 1 hour
		},
	}

	// 模拟日志（空记录器）
	global.Logger = zap.NewNop()

	// 使用项目中的模型文件初始化测试用 Casbin 实例
	enforcer, err := casbin.NewEnforcer("../../config/rbac_model.conf")
	if err != nil {
		t.Fatalf("failed to initialize casbin enforcer: %v", err)
	}

	// 添加基本测试策略规则
	// Matcher rule: g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act) || r.sub == "Admin"
	_, _ = enforcer.AddPolicy("Student", "/api/v1/workorders", "POST")
	_, _ = enforcer.AddPolicy("Student", "/api/v1/workorders", "GET")
	_, _ = enforcer.AddPolicy("Worker", "/api/v1/workorders/:id/status", "PUT")
	_, _ = enforcer.AddPolicy("Worker", "/api/v1/workorders", "GET")

	global.Enforcer = enforcer

	// 返回清理恢复函数
	return func() {
		global.Config = oldConfig
		global.Logger = oldLogger
		global.Enforcer = oldEnforcer
	}
}

// TestJWTAuth 测试 JWT 认证中间件
func TestJWTAuth(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// 生成有效 Token 的辅助函数
	validToken, err := utils.GenerateToken(1, "testuser", "Student")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// 初始化带有中间件的 Gin 路由
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
				// 解析自定义的失败返回结果
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
				// 验证 Context 中传递的属性是否正确
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

// TestCasbinRBAC 测试基于 Casbin 的 RBAC 权限控制中间件
func TestCasbinRBAC(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Setup Gin router
	r := gin.New()
	
	// 从请求头获取角色并动态设置到 Context，方便灵活测试
	r.Use(func(c *gin.Context) {
		role := c.GetHeader("Test-Role")
		if role != "" {
			c.Set("role", role)
		}
		c.Next()
	})
	
	r.Use(CasbinRBAC())
	
	// 注册代表 RESTful API 的测试处理器
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
