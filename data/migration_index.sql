-- ============================================================
-- 性能修复迁移：补充缺失索引（幂等，可重复执行）
--
-- 执行方式：本文件内含 USE 语句，只需执行一次即可同时处理两个库：
--     mysql -uroot -p < migration_index.sql
--
-- 背景：nov_novel 原先只有 PRIMARY(id)，所有榜单查询都走全表扫描 +
-- filesort。实测 30,000 本时：
--     排行榜 ORDER BY views DESC         约 12.1ms
--     最新更新 ORDER BY chapter_updated_at 约 12.8ms
--     采集去重 WHERE name = ?             约 8.7ms
-- 加索引后分别降至 0.06ms / 0.05ms / 0.02ms。
--
-- 所有语句均先检查 information_schema，重复执行不会报
-- "Duplicate key name"，可安全地反复运行。
-- ============================================================

-- ------------------------------------------------------------
-- 一、主库 gonovel.nov_novel
-- ------------------------------------------------------------
USE gonovel;

DROP PROCEDURE IF EXISTS add_novel_indexes;
DELIMITER //
CREATE PROCEDURE add_novel_indexes()
BEGIN
    -- 索引名 -> 定义（含列表达式）
    DECLARE done INT DEFAULT 0;

    -- 用临时表驱动循环，避免重复写 14 段 IF
    DROP TEMPORARY TABLE IF EXISTS tmp_idx;
    CREATE TEMPORARY TABLE tmp_idx (
        name VARCHAR(64),
        cols VARCHAR(255)
    );

    INSERT INTO tmp_idx VALUES
        ('idx_views',              'views'),
        ('idx_chapter_updated_at', 'chapter_updated_at'),
        ('idx_name',               'name'),
        ('idx_deleted_at',         'deleted_at'),
        ('idx_deleted_views',      'deleted_at, views'),
        ('idx_deleted_hot',        'deleted_at, is_hot'),
        ('idx_deleted_rec',        'deleted_at, is_rec'),
        ('idx_deleted_todayrec',   'deleted_at, is_today_rec'),
        ('idx_deleted_viprec',     'deleted_at, is_vip_rec'),
        ('idx_deleted_signnew',    'deleted_at, is_sign_new_book'),
        ('idx_deleted_collect',    'deleted_at, is_collect'),
        ('idx_deleted_cate_views', 'deleted_at, cate_id, views'),
        ('idx_deleted_chapup',     'deleted_at, chapter_updated_at');

    BLOCK: BEGIN
        DECLARE i_name VARCHAR(64);
        DECLARE i_cols VARCHAR(255);
        DECLARE cur CURSOR FOR SELECT name, cols FROM tmp_idx;
        DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

        OPEN cur;
        loop_idx: LOOP
            FETCH cur INTO i_name, i_cols;
            IF done = 1 THEN
                LEAVE loop_idx;
            END IF;

            -- 仅当索引不存在时才创建
            IF NOT EXISTS (SELECT 1 FROM information_schema.statistics
                           WHERE table_schema = 'gonovel'
                             AND table_name = 'nov_novel'
                             AND index_name = i_name) THEN
                SET @s = CONCAT('ALTER TABLE gonovel.nov_novel ADD INDEX ',
                                i_name, ' (', i_cols, ')');
                PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
            END IF;
        END LOOP;
        CLOSE cur;
    END BLOCK;

    DROP TEMPORARY TABLE IF EXISTS tmp_idx;
END //
DELIMITER ;

CALL add_novel_indexes();
DROP PROCEDURE add_novel_indexes;

-- ------------------------------------------------------------
-- 二、章节库 gochapter.nov_chapter_0000 ~ nov_chapter_0099
--
-- 已有 udx_novid_no_source(nov_id, chapter_no, source)，
-- 但采集去重走 WHERE nov_id=? AND title=?，覆盖不到 title。
-- 100 张分表逐张添加，同样做存在性检查。
-- ------------------------------------------------------------
USE gochapter;

DROP PROCEDURE IF EXISTS add_chapter_title_index;
DELIMITER //
CREATE PROCEDURE add_chapter_title_index()
BEGIN
    DECLARE i INT DEFAULT 0;
    DECLARE tbl VARCHAR(64);

    WHILE i < 100 DO
        SET tbl = CONCAT('nov_chapter_', LPAD(i, 4, '0'));

        IF EXISTS (SELECT 1 FROM information_schema.tables
                   WHERE table_schema = 'gochapter' AND table_name = tbl) THEN
            IF NOT EXISTS (SELECT 1 FROM information_schema.statistics
                           WHERE table_schema = 'gochapter' AND table_name = tbl
                             AND index_name = 'idx_novid_title') THEN
                SET @s = CONCAT('ALTER TABLE gochapter.', tbl,
                                ' ADD INDEX idx_novid_title (nov_id, title(100))');
                PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
            END IF;
        END IF;

        SET i = i + 1;
    END WHILE;
END //
DELIMITER ;

CALL add_chapter_title_index();
DROP PROCEDURE add_chapter_title_index;

-- ------------------------------------------------------------
-- 三、验证
-- ------------------------------------------------------------
-- SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
--   WHERE table_schema='gonovel' AND table_name='nov_novel';
-- SELECT COUNT(DISTINCT table_name) FROM information_schema.statistics
--   WHERE table_schema='gochapter' AND index_name='idx_novid_title';
-- EXPLAIN SELECT id,name,views FROM gonovel.nov_novel
--   WHERE deleted_at=0 ORDER BY views DESC LIMIT 9;
