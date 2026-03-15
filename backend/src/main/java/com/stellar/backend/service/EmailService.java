package com.stellar.backend.service;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.mail.javamail.JavaMailSender;
import org.springframework.mail.javamail.MimeMessageHelper;
import org.springframework.stereotype.Service;

import jakarta.mail.MessagingException;
import jakarta.mail.internet.MimeMessage;
import java.util.Random;
import java.util.concurrent.TimeUnit;

/**
 * 邮件验证码服务
 * 验证码存储在 Redis 中，5 分钟过期
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class EmailService {

    private final JavaMailSender mailSender;
    private final StringRedisTemplate redisTemplate;

    @Value("${spring.mail.username}")
    private String fromEmail;

    private static final String CODE_PREFIX = "email:code:";
    private static final long CODE_EXPIRE_MINUTES = 5;
    private static final int CODE_LENGTH = 6;

    /**
     * 发送验证码
     */
    public void sendVerificationCode(String toEmail) {
        String code = generateCode();

        // 存入 Redis，5 分钟过期
        redisTemplate.opsForValue().set(
                CODE_PREFIX + toEmail,
                code,
                CODE_EXPIRE_MINUTES,
                TimeUnit.MINUTES
        );

        // 发送邮件
        try {
            MimeMessage message = mailSender.createMimeMessage();
            MimeMessageHelper helper = new MimeMessageHelper(message, true, "UTF-8");
            helper.setFrom(fromEmail);
            helper.setTo(toEmail);
            helper.setSubject("【Stellar Ionosphere】邮箱验证码");
            helper.setText(buildEmailContent(code), true);
            mailSender.send(message);
            log.info("验证码已发送至 {}", toEmail);
        } catch (MessagingException e) {
            log.error("发送验证码失败: {}", e.getMessage());
            throw new RuntimeException("验证码发送失败，请稍后重试");
        }
    }

    /**
     * 校验验证码
     */
    public boolean verifyCode(String email, String code) {
        String key = CODE_PREFIX + email;
        String cachedCode = redisTemplate.opsForValue().get(key);
        if (cachedCode != null && cachedCode.equals(code)) {
            redisTemplate.delete(key); // 验证成功后删除
            return true;
        }
        return false;
    }

    private String generateCode() {
        Random random = new Random();
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < CODE_LENGTH; i++) {
            sb.append(random.nextInt(10));
        }
        return sb.toString();
    }

    private String buildEmailContent(String code) {
        return """
            <div style="max-width:420px;margin:0 auto;font-family:'Space Grotesk',sans-serif;">
              <div style="border:4px solid #000;background:#fff;padding:32px;">
                <div style="background:#FFD93D;border:4px solid #000;padding:8px 16px;display:inline-block;
                            font-size:12px;font-weight:700;letter-spacing:0.2em;text-transform:uppercase;
                            box-shadow:4px 4px 0 0 #000;margin-bottom:24px;">
                  STELLAR IONOSPHERE
                </div>
                <h1 style="font-size:32px;font-weight:700;margin:16px 0 8px;text-transform:uppercase;">
                  验证码
                </h1>
                <p style="font-size:15px;color:#000;margin-bottom:24px;">
                  你的邮箱验证码是：
                </p>
                <div style="background:#FF6B6B;border:4px solid #000;padding:16px;text-align:center;
                            font-size:36px;font-weight:700;letter-spacing:12px;color:#fff;
                            box-shadow:8px 8px 0 0 #000;">
                  %s
                </div>
                <p style="font-size:13px;color:#666;margin-top:20px;">
                  验证码 %d 分钟内有效，请勿泄露给他人。
                </p>
              </div>
            </div>
            """.formatted(code, CODE_EXPIRE_MINUTES);
    }
}
