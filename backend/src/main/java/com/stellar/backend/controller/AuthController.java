package com.stellar.backend.controller;

import com.stellar.backend.dto.AuthResponse;
import com.stellar.backend.dto.LoginRequest;
import com.stellar.backend.dto.RegisterRequest;
import com.stellar.backend.entity.User;
import com.stellar.backend.service.EmailService;
import com.stellar.backend.service.UserService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

/**
 * 认证控制器
 */
@RestController
@RequestMapping("/api/auth")
@RequiredArgsConstructor
public class AuthController {

    private final UserService userService;
    private final EmailService emailService;

    /**
     * 发送邮箱验证码
     */
    @PostMapping("/send-code")
    public ResponseEntity<?> sendCode(@RequestBody Map<String, String> body) {
        String email = body.get("email");
        if (email == null || email.isBlank()) {
            throw new RuntimeException("邮箱不能为空");
        }
        emailService.sendVerificationCode(email);
        return ResponseEntity.ok(Map.of("message", "验证码已发送"));
    }

    /**
     * 注册
     */
    @PostMapping("/register")
    public ResponseEntity<AuthResponse> register(@Valid @RequestBody RegisterRequest request) {
        AuthResponse response = userService.register(request);
        return ResponseEntity.ok(response);
    }

    /**
     * 登录
     */
    @PostMapping("/login")
    public ResponseEntity<AuthResponse> login(@Valid @RequestBody LoginRequest request) {
        AuthResponse response = userService.login(request);
        return ResponseEntity.ok(response);
    }

    /**
     * 获取当前登录用户信息（需要 Token）
     */
    @GetMapping("/me")
    public ResponseEntity<?> getCurrentUser(Authentication authentication) {
        String email = authentication.getName();
        User user = userService.getUserByEmail(email);
        if (user == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok(Map.of(
                "id", user.getId(),
                "username", user.getUsername(),
                "email", user.getEmail(),
                "emailVerified", user.getEmailVerified(),
                "createdAt", user.getCreatedAt().toString()
        ));
    }
}
