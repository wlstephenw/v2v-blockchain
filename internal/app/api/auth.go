package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
)

// JWT 密钥（生产环境应从配置文件或环境变量读取）
var jwtSecret = []byte("v2v-blockchain-secret-key-change-in-production")

// Token 有效期（24小时）
const tokenExpiration = 24 * time.Hour

// Claims 定义 JWT Claims 结构
type Claims struct {
	VehicleID string `json:"vehicle_id"`
	Address   string `json:"address"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// contextKey 用于从 context 中获取用户信息
type contextKey string

const (
	// ContextKeyClaims 用于存储 JWT claims 的 context key
	ContextKeyClaims contextKey = "claims"
	// ContextKeyVehicleID 用于存储车辆ID的 context key
	ContextKeyVehicleID contextKey = "vehicle_id"
)

// GenerateToken 生成 JWT Token
func GenerateToken(vehicleID string, address blockchain.Address, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		VehicleID: vehicleID,
		Address:   address.String(),
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "v2v-blockchain",
			Subject:   vehicleID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken 解析并验证 JWT Token
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// authMiddleware JWT 认证中间件
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从 Header 获取 Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.errorResponse(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		// 验证 Bearer token 格式
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			s.errorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		tokenString := parts[1]

		// 解析并验证 token
		claims, err := ParseToken(tokenString)
		if err != nil {
			s.errorResponse(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// 将 claims 添加到 context
		ctx := context.WithValue(r.Context(), ContextKeyClaims, claims)
		ctx = context.WithValue(ctx, ContextKeyVehicleID, claims.VehicleID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// optionalAuthMiddleware 可选的认证中间件（不强制要求认证，但如果提供了 token 会解析）
func (s *Server) optionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			next.ServeHTTP(w, r)
			return
		}

		tokenString := parts[1]
		claims, err := ParseToken(tokenString)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyClaims, claims)
		ctx = context.WithValue(ctx, ContextKeyVehicleID, claims.VehicleID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClaimsFromContext 从 context 获取 JWT claims
func GetClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ContextKeyClaims).(*Claims)
	return claims, ok
}

// GetVehicleIDFromContext 从 context 获取车辆ID
func GetVehicleIDFromContext(ctx context.Context) (string, bool) {
	vehicleID, ok := ctx.Value(ContextKeyVehicleID).(string)
	return vehicleID, ok
}

// requireRole 检查用户角色
func requireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaimsFromContext(r.Context())
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "unauthorized",
				})
				return
			}

			for _, role := range allowedRoles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "forbidden: insufficient permissions",
			})
		})
	}
}
