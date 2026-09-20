-- ============================================================
-- 页面性能统计：SQL 次数开关
--
-- 页面底部会显示「本条 12.3ms ｜ SQL 8 次 3.1ms」这样的统计，
-- 便于直观定位性能瓶颈。
--
-- 说明：SQL 次数统计默认关闭。原因：beego ORM 只有在 orm.Debug=true 时
-- 才会记录查询日志，而该模式下每条 SQL 都会做一次完整的字符串格式化
-- （拼出完整 SQL 与全部参数），在生产环境会带来可观开销，反而拖慢页面。
-- 需要排查「某页到底查了多少次库」时把本项设为 1 即可。
--
-- 特性：幂等
-- 执行：mysql -unovel -p'novel@2024' -h127.0.0.1 -P3307 gonovel < migration_perf.sql
-- ============================================================

USE gonovel;

INSERT IGNORE INTO nov_config (`key`, `value`, `created_at`, `updated_at`)
VALUES ('PerfSQLCount', '0', UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- 验证
-- SELECT `key`, `value` FROM nov_config WHERE `key` = 'PerfSQLCount';
