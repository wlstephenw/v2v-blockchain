package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
)

// TestAuthMiddleware 测试认证中间件
func TestAuthMiddleware(t *testing.T) {
	// 创建一个简化版服务器
	server := &Server{
		rateLimiter: NewRateLimiter(1000),
		clients:     make(map[*websocket.Conn]bool),
		broadcast:   make(chan interface{}, 100),
		stopCh:      make(chan struct{}),
	}
	defer server.rateLimiter.Stop()

	// 创建一个简单的测试路由
	router := http.NewServeMux()

	// 包装认证中间件
	authHandler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	router.Handle("/protected", authHandler)

	t.Run("missing auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid auth format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "invalid-token")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("valid token", func(t *testing.T) {
		// 生成有效 token（20字节地址 = 40个十六进制字符）
		address, err := blockchain.HexToAddress("1234567890abcdef1234567890abcdef12345678")
		require.NoError(t, err)
		token, err := GenerateToken("vehicle-1", address, "validator")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// TestOptionalAuthMiddleware 测试可选认证中间件
func TestOptionalAuthMiddleware(t *testing.T) {
	server := &Server{
		rateLimiter: NewRateLimiter(1000),
		clients:     make(map[*websocket.Conn]bool),
		broadcast:   make(chan interface{}, 100),
		stopCh:      make(chan struct{}),
	}
	defer server.rateLimiter.Stop()

	router := http.NewServeMux()

	// 包装可选认证中间件
	optionalHandler := server.optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查 context 中是否有 claims
		claims, hasClaims := GetClaimsFromContext(r.Context())
		if hasClaims {
			json.NewEncoder(w).Encode(map[string]string{
				"status":     "ok",
				"vehicle_id": claims.VehicleID,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]string{
				"status": "ok",
			})
		}
	}))

	router.Handle("/optional", optionalHandler)

	t.Run("no token provided", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/optional", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp["status"])
		assert.Empty(t, resp["vehicle_id"])
	})

	t.Run("valid token provided", func(t *testing.T) {
		address, err := blockchain.HexToAddress("1234567890abcdef1234567890abcdef12345678")
		require.NoError(t, err)
		token, err := GenerateToken("vehicle-123", address, "validator")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/optional", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]string
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp["status"])
		assert.Equal(t, "vehicle-123", resp["vehicle_id"])
	})
}

// TestGenerateAndParseToken 测试 Token 生成和解析
func TestGenerateAndParseToken(t *testing.T) {
	address, err := blockchain.HexToAddress("1234567890abcdef1234567890abcdef12345678")
	require.NoError(t, err)

	t.Run("generate and parse valid token", func(t *testing.T) {
		token, err := GenerateToken("vehicle-1", address, "validator")
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := ParseToken(token)
		require.NoError(t, err)
		assert.Equal(t, "vehicle-1", claims.VehicleID)
		assert.Equal(t, address.String(), claims.Address)
		assert.Equal(t, "validator", claims.Role)
	})

	t.Run("parse invalid token", func(t *testing.T) {
		_, err := ParseToken("invalid-token")
		assert.Error(t, err)
	})

	t.Run("parse empty token", func(t *testing.T) {
		_, err := ParseToken("")
		assert.Error(t, err)
	})

	t.Run("parse malformed token", func(t *testing.T) {
		_, err := ParseToken("Bearer invalid.token.here")
		assert.Error(t, err)
	})
}

// TestGetVehicleIDFromContext 测试从 context 获取车辆ID
func TestGetVehicleIDFromContext(t *testing.T) {
	address, err := blockchain.HexToAddress("1234567890abcdef1234567890abcdef12345678")
	require.NoError(t, err)

	token, err := GenerateToken("vehicle-999", address, "follower")
	require.NoError(t, err)

	claims, err := ParseToken(token)
	require.NoError(t, err)

	server := &Server{
		rateLimiter: NewRateLimiter(1000),
		clients:     make(map[*websocket.Conn]bool),
		broadcast:   make(chan interface{}, 100),
		stopCh:      make(chan struct{}),
	}
	defer server.rateLimiter.Stop()

	router := http.NewServeMux()
	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vehicleID, ok := GetVehicleIDFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "vehicle-999", vehicleID)

		retrievedClaims, ok := GetClaimsFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, claims.VehicleID, retrievedClaims.VehicleID)
		assert.Equal(t, claims.Address, retrievedClaims.Address)
		assert.Equal(t, claims.Role, retrievedClaims.Role)

		w.WriteHeader(http.StatusOK)
	}))
	router.Handle("/test", handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestRateLimiter 测试限流器
func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(60) // 每分钟60请求
	defer rl.Stop()

	t.Run("allow requests within limit", func(t *testing.T) {
		ip := "192.168.1.1"
		// 前60个请求应该被允许
		for i := 0; i < 60; i++ {
			allowed, _, _ := rl.allow(ip)
			assert.True(t, allowed, "request %d should be allowed", i)
		}
	})

	t.Run("block requests over limit", func(t *testing.T) {
		ip := "192.168.1.2"
		// 耗尽令牌
		for i := 0; i < 60; i++ {
			rl.allow(ip)
		}
		// 下一个请求应该被拒绝
		allowed, _, _ := rl.allow(ip)
		assert.False(t, allowed)
	})

	t.Run("different IPs have separate limits", func(t *testing.T) {
		// 耗尽 IP1 的令牌
		for i := 0; i < 60; i++ {
			rl.allow("192.168.1.3")
		}

		// IP2 应该有独立的限额
		allowed, _, _ := rl.allow("192.168.1.4")
		assert.True(t, allowed)
	})

	t.Run("get client IP from request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		ip := getClientIP(req)
		assert.Equal(t, "192.168.1.100", ip)
	})

	t.Run("get client IP from X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", "203.0.113.1")

		ip := getClientIP(req)
		assert.Equal(t, "203.0.113.1", ip)
	})

	t.Run("get client IP from X-Real-IP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		req.Header.Set("X-Real-IP", "198.51.100.1")

		ip := getClientIP(req)
		assert.Equal(t, "198.51.100.1", ip)
	})
}

// TestRateLimitMiddleware 测试限流中间件
func TestRateLimitMiddleware(t *testing.T) {
	server := &Server{
		rateLimiter: NewRateLimiter(1000),
	}
	defer server.rateLimiter.Stop()

	handler := server.rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	t.Run("rate limit headers present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "1000", rr.Header().Get("X-RateLimit-Limit"))
		assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("rate limit exceeded returns 429", func(t *testing.T) {
		// 创建低限流的限流器
		server := &Server{
			rateLimiter: NewRateLimiter(1), // 每分钟1请求
		}
		defer server.rateLimiter.Stop()

		handler := server.rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// 第一个请求应该成功
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.RemoteAddr = "192.168.1.50:12345"
		rr1 := httptest.NewRecorder()
		handler.ServeHTTP(rr1, req1)
		assert.Equal(t, http.StatusOK, rr1.Code)

		// 第二个请求应该被限流
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.RemoteAddr = "192.168.1.50:12345"
		rr2 := httptest.NewRecorder()
		handler.ServeHTTP(rr2, req2)
		assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
		assert.NotEmpty(t, rr2.Header().Get("Retry-After"))
	})
}

// TestCORS 测试 CORS 中间件
func TestCORS(t *testing.T) {
	server := &Server{}

	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("CORS headers present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Methods"))
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("CORS preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/transactions", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// TestRecoveryMiddleware 测试恢复中间件
func TestRecoveryMiddleware(t *testing.T) {
	server := &Server{}

	handler := server.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	t.Run("recovers from panic", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var resp map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "internal server error", resp["error"])
	})
}

// TestLoggingMiddleware 测试日志中间件
func TestLoggingMiddleware(t *testing.T) {
	server := &Server{}

	handler := server.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		time.Sleep(1 * time.Millisecond) // 模拟处理时间
	}))

	t.Run("logs request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		// 日志输出会打印到标准输出/错误，这里只验证请求能通过
	})
}

// TestResponseHelpers 测试响应辅助函数
func TestResponseHelpers(t *testing.T) {
	server := &Server{}

	t.Run("jsonResponse", func(t *testing.T) {
		rr := httptest.NewRecorder()
		data := map[string]string{"key": "value", "foo": "bar"}

		server.jsonResponse(rr, http.StatusCreated, data)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var resp map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "value", resp["key"])
		assert.Equal(t, "bar", resp["foo"])
	})

	t.Run("errorResponse", func(t *testing.T) {
		rr := httptest.NewRecorder()

		server.errorResponse(rr, http.StatusBadRequest, "invalid input")

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var resp map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "invalid input", resp["error"])
	})
}
