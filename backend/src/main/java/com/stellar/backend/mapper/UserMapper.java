package com.stellar.backend.mapper;

import com.stellar.backend.entity.User;
import org.apache.ibatis.annotations.*;

/**
 * 用户 Mapper
 */
@Mapper
public interface UserMapper {

    @Select("SELECT * FROM `user` WHERE email = #{email}")
    User findByEmail(@Param("email") String email);

    @Select("SELECT * FROM `user` WHERE username = #{username}")
    User findByUsername(@Param("username") String username);

    @Select("SELECT * FROM `user` WHERE id = #{id}")
    User findById(@Param("id") Long id);

    @Insert("INSERT INTO `user` (username, email, password_hash) VALUES (#{username}, #{email}, #{passwordHash})")
    @Options(useGeneratedKeys = true, keyProperty = "id")
    int insert(User user);

    @Update("UPDATE `user` SET email_verified = #{emailVerified} WHERE id = #{id}")
    int updateEmailVerified(@Param("id") Long id, @Param("emailVerified") Boolean emailVerified);
}
