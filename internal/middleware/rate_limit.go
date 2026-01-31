package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RateLimitMiddleware(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := "rate_limit:" + clientIP

		ctx := c.Request.Context()

		// Use Redis INCR with expiry
		current, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			c.Abort()
			return
		}

		if current == 1 {
			// Set expiry on first request
			rdb.Expire(ctx, key, window)
		}

		if current > int64(limit) {
			ttl, _ := rdb.TTL(ctx, key).Result()
			c.Header("Retry-After", strconv.FormatInt(int64(ttl/time.Second), 10))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": ttl.Seconds(),
			})
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(int64(limit)-current, 10))
		c.Next()
	}
}
