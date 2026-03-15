-- Stellar Ionosphere 用户表
-- 使用前请先创建数据库: CREATE DATABASE IF NOT EXISTS stellar DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user` (
    `id`             BIGINT       PRIMARY KEY AUTO_INCREMENT,
    `username`       VARCHAR(64)  NOT NULL COMMENT '用户名',
    `email`          VARCHAR(128) NOT NULL COMMENT '邮箱',
    `password_hash`  VARCHAR(256) NOT NULL COMMENT '密码哈希（BCrypt）',
    `avatar_url`     VARCHAR(512)          COMMENT '头像地址',
    `email_verified` TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '邮箱是否验证 0-否 1-是',
    `status`         TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '账号状态 0-禁用 1-正常',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `uk_email` (`email`),
    UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';
