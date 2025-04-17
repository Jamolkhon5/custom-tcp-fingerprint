#!/bin/bash
set -e

if [ "$(id -u)" -ne 0 ]; then
  echo "Этот скрипт должен запускаться с правами суперпользователя (sudo)"
  exit 1
fi

if [ ! -c /dev/net/tun ]; then
  mkdir -p /dev/net
  mknod /dev/net/tun c 10 200
  chmod 600 /dev/net/tun
fi

echo 1 > /proc/sys/net/ipv4/ip_forward

for cmd in iptables ip tcpdump wireshark tshark; do
  if ! command -v $cmd &> /dev/null; then
    echo "Ошибка: утилита $cmd не найдена. Пожалуйста, установите ее."
    exit 1
  fi
done

echo "Среда настроена успешно."