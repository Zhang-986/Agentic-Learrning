package com.stellar.backend.service;

import com.stellar.backend.dto.AuthResponse;
import com.stellar.backend.dto.LoginRequest;
import com.stellar.backend.dto.RegisterRequest;
import com.stellar.backend.entity.User;
import com.stellar.backend.mapper.UserMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;

/**
 * 用户认证服务
 */
@Service
@RequiredArgsConstructor
public class UserService {

    private final UserMapper userMapper;
    private final PasswordEncoder passwordEncoder;
    private final JwtService jwtService;
    private final EmailService emailService;

    /**
     * 用户注册（需先验证邮箱验证码）
     */
    public AuthResponse register(RegisterRequest request) {
        // 1. 校验验证码
        if (!emailService.verifyCode(request.getEmail(), request.getCode())) {
            throw new RuntimeException("验证码错误或已过期");
        }

        // 2. 检查邮箱是否已注册
        if (userMapper.findByEmail(request.getEmail()) != null) {
            throw new RuntimeException("该邮箱已被注册");
        }

        // 3. 检查用户名是否已存在
        if (userMapper.findByUsername(request.getUsername()) != null) {
            throw new RuntimeException("该用户名已被使用");
        }

        // 4. 创建用户（邮箱已验证）
        User user = new User();
        user.setUsername(request.getUsername());
        user.setEmail(request.getEmail());
        user.setPasswordHash(passwordEncoder.encode(request.getPassword()));
        user.setEmailVerified(true);
        userMapper.insert(user);

        // 5. 生成 Token
        String token = jwtService.generateToken(user.getEmail());

        return AuthResponse.builder()
                .token(token)
                .username(user.getUsername())
                .email(user.getEmail())
                .build();
    }

    /**
     * 用户登录
     */
    public AuthResponse login(LoginRequest request) {
        User user = userMapper.findByEmail(request.getEmail());
        if (user == null) {
            throw new RuntimeException("邮箱或密码错误");
        }

        if (!passwordEncoder.matches(request.getPassword(), user.getPasswordHash())) {
            throw new RuntimeException("邮箱或密码错误");
        }

        if (user.getStatus() == 0) {
            throw new RuntimeException("该账号已被禁用");
        }

        String token = jwtService.generateToken(user.getEmail());

        return AuthResponse.builder()
                .token(token)
                .username(user.getUsername())
                .email(user.getEmail())
                .build();
    }

    /**
     * 根据邮箱获取用户
     */
    public User getUserByEmail(String email) {
        return userMapper.findByEmail(email);
    }
}
