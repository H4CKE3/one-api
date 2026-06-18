package model

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
)

var channelRateLimitCooldownStore sync.Map

func rateLimitCooldownKey(channelID int) string {
	return fmt.Sprintf("channel_rate_limit:%d", channelID)
}

func MarkChannelRateLimited(channelID int, cooldown time.Duration) {
	if cooldown <= 0 {
		cooldown = time.Duration(config.ChannelRateLimitCooldownSeconds) * time.Second
	}
	if cooldown <= 0 {
		return
	}
	expireAt := time.Now().Add(cooldown)
	channelRateLimitCooldownStore.Store(channelID, expireAt)
	if common.RedisEnabled {
		_ = common.RedisSet(rateLimitCooldownKey(channelID), strconv.FormatInt(expireAt.UnixMilli(), 10), cooldown)
	}
}

func IsChannelRateLimited(channelID int) bool {
	if config.ChannelRateLimitCooldownSeconds <= 0 {
		return false
	}
	if value, ok := channelRateLimitCooldownStore.Load(channelID); ok {
		if expireAt, ok := value.(time.Time); ok {
			if time.Now().Before(expireAt) {
				return true
			}
			channelRateLimitCooldownStore.Delete(channelID)
		}
	}
	if !common.RedisEnabled {
		return false
	}
	expireAtStr, err := common.RedisGet(rateLimitCooldownKey(channelID))
	if err != nil || expireAtStr == "" {
		return false
	}
	expireAtMillis, err := strconv.ParseInt(expireAtStr, 10, 64)
	if err != nil {
		return false
	}
	expireAt := time.UnixMilli(expireAtMillis)
	if time.Now().Before(expireAt) {
		channelRateLimitCooldownStore.Store(channelID, expireAt)
		return true
	}
	_ = common.RedisDel(rateLimitCooldownKey(channelID))
	return false
}
