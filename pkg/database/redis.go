package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/meet-app/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

// InitRedis initializes Redis connection
func InitRedis(cfg *config.RedisConfig) error {
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Ping Redis to check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	RedisClient = client
	log.Println("✅ Redis connected successfully")
	return nil
}

// CloseRedis closes the Redis connection
func CloseRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}

// GetRedis returns the Redis client instance
func GetRedis() *redis.Client {
	return RedisClient
}

// Publish publishes a message to a Redis channel
func Publish(ctx context.Context, channel string, message interface{}) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return RedisClient.Publish(ctx, channel, message).Err()
}

// Subscribe subscribes to a Redis channel
func Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	if RedisClient == nil {
		return nil
	}
	return RedisClient.Subscribe(ctx, channels...)
}
