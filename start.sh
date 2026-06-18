#!/bin/bash

cd $(dirname $0)

# JWT 密钥（生产环境必须修改）
export JWT_SECRET='your-very-secure-jwt-secret-key'

# 管理员账号
export ADMIN_USERNAME='admin'

# 管理员密码（设置其中一个即可）
export ADMIN_PASSWORD='admin'

# 或者使用密码哈希（推荐生产环境使用）
#export ADMIN_PASSWORD_HASH='$2a$10$your-bcrypt-hash-here'

go mod tidy && go build . && ./easy-notes
