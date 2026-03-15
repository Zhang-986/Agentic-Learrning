package com.stellar.backend.entity;

import lombok.Data;
import java.time.LocalDateTime;

/**
 * 用户实体
 */
@Data
public class User {
    private Long id;
    private String username;
    private String email;
    private String passwordHash;
    private String avatarUrl;
    private Boolean emailVerified;
    private Integer status;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
