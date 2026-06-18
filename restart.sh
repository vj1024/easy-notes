#!/bin/bash

cd $(dirname $0)

PROCESS_NAME=easy-notes
PROCESS_PATH=$(pwd)/$PROCESS_NAME
PROCESS_ARGS=

# 进程全路径且精确匹配（不包含参数）
function findProcess() {
  pgrep -a -f "$PROCESS_PATH" | awk -v path="$PROCESS_PATH" '{ if($2 == path) print $0 }'
}

# 检查并杀掉进程
processes=$(findProcess)
if [[ "$processes" != "" ]]; then
  echo -e "### Kill processes:\n$processes\n"
  echo "$processes" | awk '{print $1}' | xargs kill
  sleep 1

  # 确认已经被kill
  processes=$(findProcess)
  if [[ "$processes" != "" ]]; then
    echo -e "### Kill processes failed:\n$processes\n"
    echo "You can try force-killing the process using kill -9."
    exit 1
  fi
fi

# JWT 密钥（生产环境必须修改）
export JWT_SECRET='your-very-secure-jwt-secret-key'

# 管理员账号
export ADMIN_USERNAME='admin'

# 管理员密码（设置其中一个即可）
export ADMIN_PASSWORD='admin'

# 或者使用密码哈希（推荐生产环境使用）
#export ADMIN_PASSWORD_HASH='$2a$10$your-bcrypt-hash-here'

# 启动进程
echo -e "### Start process:\n$PROCESS_PATH $PROCESS_ARGS\n"
nohup $PROCESS_PATH $PROCESS_ARGS >>log.txt 2>&1 &
sleep 1

echo -e "### Current process:\n$(findProcess)"
