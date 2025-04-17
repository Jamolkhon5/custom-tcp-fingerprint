#!/bin/bash
set -e

if [ "$#" -ne 3 ]; then
  echo "Использование: $0 <tun_name> <target_host> <local_port>"
  exit 1
fi

TUN_NAME=$1
TARGET_HOST=$2
LOCAL_PORT=$3
MARK_VALUE="0x1337"

if [ "$(id -u)" -ne 0 ]; then
  echo "Этот скрипт должен запускаться с правами суперпользователя (sudo)"
  exit 1
fi

add_iptables_rules() {
  iptables -t filter -A INPUT -p tcp --dport $LOCAL_PORT -j ACCEPT

  iptables -t mangle -A OUTPUT -p tcp --sport $LOCAL_PORT -j MARK --set-mark $MARK_VALUE

  iptables -t mangle -A OUTPUT -p tcp -d $TARGET_HOST -j MARK --set-mark $MARK_VALUE

  iptables -t filter -A FORWARD -i lo -o $TUN_NAME -m mark --mark $MARK_VALUE -j ACCEPT
  iptables -t filter -A FORWARD -i $TUN_NAME -o lo -m mark --mark $MARK_VALUE -j ACCEPT

  iptables -t nat -A POSTROUTING -o $TUN_NAME -j MASQUERADE

  echo "Правила iptables успешно добавлены"
}

remove_iptables_rules() {
  iptables -t nat -D POSTROUTING -o $TUN_NAME -j MASQUERADE 2>/dev/null || true

  iptables -t filter -D FORWARD -i $TUN_NAME -o lo -m mark --mark $MARK_VALUE -j ACCEPT 2>/dev/null || true
  iptables -t filter -D FORWARD -i lo -o $TUN_NAME -m mark --mark $MARK_VALUE -j ACCEPT 2>/dev/null || true

  iptables -t mangle -D OUTPUT -p tcp -d $TARGET_HOST -j MARK --set-mark $MARK_VALUE 2>/dev/null || true
  iptables -t mangle -D OUTPUT -p tcp --sport $LOCAL_PORT -j MARK --set-mark $MARK_VALUE 2>/dev/null || true

  iptables -t filter -D INPUT -p tcp --dport $LOCAL_PORT -j ACCEPT 2>/dev/null || true

  echo "Правила iptables успешно удалены"
}

case "$4" in
  "add"|"")
    remove_iptables_rules
    add_iptables_rules
    ;;
  "remove")
    remove_iptables_rules
    ;;
  *)
    echo "Неизвестная операция: $4. Используйте 'add' или 'remove'"
    exit 1
    ;;
esac

exit 0