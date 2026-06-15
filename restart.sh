#!/bin/bash

cd $(dirname $0)

PROCESS_NAME=easy-notes

# 检查并杀掉进程
if pgrep -f "$PROCESS_NAME" >/dev/null; then
  echo "Stopping $PROCESS_NAME..."
  pkill -f "$PROCESS_NAME"
  sleep 1
  pkill -9 -f "$PROCESS_NAME" 2>/dev/null
fi

# 启动进程
echo "Starting $PROCESS_NAME..."
nohup ./start.sh >std.log 2>&1 &
sleep 1
echo "Restarted with PID: $(pgrep -f "$PROCESS_NAME")"
