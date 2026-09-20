-- ============================================================
-- 新增采集节流配置项
--
-- 背景：源站（5566xs）对高频采集会返回 502 限流，实测「同一秒内
-- 最多 15 个请求」打向同一域名即触发。现增加两个可在后台调节的参数，
-- 用于控制采集节奏，无需改代码即可调速。
--
-- 说明：ConfigService.Set() 只会更新「已存在」的键（键不存在直接返回），
-- 因此必须先在 nov_config 表插入记录，后台设置页才能保存生效。
--
-- 特性：幂等。nov_config.key 有唯一索引 udx_key，配合 INSERT IGNORE
--       可重复执行而不报错、不产生重复行。
-- 执行：mysql -unovel -p'novel@2024' -h127.0.0.1 -P3307 gonovel < migration_snatch_pace.sql
-- ============================================================

USE gonovel;

-- 采集请求最小间隔（毫秒）
-- 1500ms 相比原始实现已是明显放缓，可大幅降低被限流概率
INSERT IGNORE INTO nov_config (`key`, `value`, `created_at`, `updated_at`)
VALUES ('SnatchInterval', '1500', UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- 采集随机抖动（毫秒）：在间隔之上附加随机等待，避免固定节奏被识别为爬虫
INSERT IGNORE INTO nov_config (`key`, `value`, `created_at`, `updated_at`)
VALUES ('SnatchJitter', '800', UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- 验证
-- SELECT `key`, `value` FROM nov_config WHERE `key` IN ('SnatchInterval','SnatchJitter');
