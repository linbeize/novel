-- ============================================================
-- 新增「分批入库」配置项
--
-- 背景：原实现把所有章节累积在内存、最后一次性写入数据库。采集
-- 《诡秘之主》(1453 章、约 2.3 小时) 时，若中途进程退出或被源站限流
-- 打断，已采集的章节会全部丢失（实测出现 chapter_num 一直为 0）。
--
-- 改为每累计 N 章落库一次，即使中途失败也能保留进度，并支持下次
-- 断点续采。此参数控制批次大小。
--
-- 特性：幂等（nov_config.key 有唯一索引 udx_key）
-- 执行：mysql -unovel -p'novel@2024' -h127.0.0.1 -P3307 gonovel < migration_batch_size.sql
-- ============================================================

USE gonovel;

-- 分批入库批次大小（章）。100 章 ≈ 300 次请求 ≈ 10 分钟落库一次
INSERT IGNORE INTO nov_config (`key`, `value`, `created_at`, `updated_at`)
VALUES ('BatchSize', '100', UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- 验证
-- SELECT `key`, `value` FROM nov_config WHERE `key` = 'BatchSize';
