-- 为渠道表添加错误统计字段
-- 添加error_count字段（错误请求次数）
ALTER TABLE channels ADD COLUMN IF NOT EXISTS error_count BIGINT DEFAULT 0;

-- 添加total_count字段（总请求次数）
ALTER TABLE channels ADD COLUMN IF NOT EXISTS total_count BIGINT DEFAULT 0;

-- 为已有数据初始化字段
UPDATE channels SET error_count = 0 WHERE error_count IS NULL;
UPDATE channels SET total_count = 0 WHERE total_count IS NULL;

-- 添加索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_chat_records_channel_status ON chat_records(channel_id, status);
CREATE INDEX IF NOT EXISTS idx_channels_error_count ON channels(error_count);
CREATE INDEX IF NOT EXISTS idx_channels_total_count ON channels(total_count);

