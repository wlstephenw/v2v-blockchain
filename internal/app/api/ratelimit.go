package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter 基于令牌桶算法的限流器
type RateLimiter struct {
	// 每IP的限流器映射
	clients map[string]*ClientLimiter
	mu      sync.RWMutex

	// 限流配置
	requestsPerMinute int
	burstSize         int

	// 清理周期
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

// ClientLimiter 单个客户端的限流器
type ClientLimiter struct {
	// 令牌数量
	tokens float64
	// 上次更新时间
	lastUpdate time.Time
	// 互斥锁
	mu sync.Mutex
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 1000 // 默认每分钟1000请求
	}

	rl := &RateLimiter{
		clients:           make(map[string]*ClientLimiter),
		requestsPerMinute: requestsPerMinute,
		burstSize:         requestsPerMinute, // 突发容量等于每分钟请求数
		cleanupInterval:   5 * time.Minute,   // 每5分钟清理一次
		stopCh:            make(chan struct{}),
	}

	// 启动清理 goroutine
	go rl.cleanup()

	return rl
}

// Stop 停止限流器
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// getClientIP 从请求中获取客户端 IP
func getClientIP(r *http.Request) string {
	// 检查 X-Forwarded-For 头（代理后面）
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// 取第一个 IP
		ip := net.ParseIP(xff)
		if ip != nil {
			return ip.String()
		}
	}

	// 检查 X-Real-IP 头
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		ip := net.ParseIP(xri)
		if ip != nil {
			return ip.String()
		}
	}

	// 从 RemoteAddr 解析
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// getOrCreateLimiter 获取或创建客户端限流器
func (rl *RateLimiter) getOrCreateLimiter(ip string) *ClientLimiter {
	rl.mu.RLock()
	limiter, exists := rl.clients[ip]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 双重检查
	if limiter, exists := rl.clients[ip]; exists {
		return limiter
	}

	limiter = &ClientLimiter{
		tokens:     float64(rl.burstSize), // 初始满令牌
		lastUpdate: time.Now(),
	}
	rl.clients[ip] = limiter

	return limiter
}

// allow 检查是否允许请求
func (rl *RateLimiter) allow(ip string) (bool, int, time.Duration) {
	limiter := rl.getOrCreateLimiter(ip)

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(limiter.lastUpdate)
	limiter.lastUpdate = now

	// 计算令牌生成速率（每分钟 requestsPerMinute）
	rate := float64(rl.requestsPerMinute) / 60.0 // 每秒生成的令牌数

	// 添加新令牌
	limiter.tokens += elapsed.Seconds() * rate
	if limiter.tokens > float64(rl.burstSize) {
		limiter.tokens = float64(rl.burstSize)
	}

	// 检查是否有足够令牌
	if limiter.tokens >= 1 {
		limiter.tokens--
		remaining := int(limiter.tokens)
		// 计算重置时间（当令牌满时）
		resetAfter := time.Duration((float64(rl.burstSize)-limiter.tokens)/rate) * time.Second
		return true, remaining, resetAfter
	}

	// 没有令牌，计算需要等待的时间
	waitTime := time.Duration((1-limiter.tokens)/rate) * time.Second
	return false, 0, waitTime
}

// cleanup 定期清理不活跃的客户端
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, limiter := range rl.clients {
				limiter.mu.Lock()
				// 如果超过10分钟没有活动，删除
				if now.Sub(limiter.lastUpdate) > 10*time.Minute {
					delete(rl.clients, ip)
				}
				limiter.mu.Unlock()
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// rateLimitMiddleware 限流中间件
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.rateLimiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		ip := getClientIP(r)
		allowed, remaining, retryAfter := s.rateLimiter.allow(ip)

		// 设置限流响应头
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(s.rateLimiter.requestsPerMinute))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter.Seconds(),
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
