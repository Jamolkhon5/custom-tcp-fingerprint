#!/bin/bash
set -e

if [ "$#" -lt 2 ]; then
  echo "Использование: $0 <interface_name> <output_file> [duration_seconds]"
  exit 1
fi

INTERFACE=$1
OUTPUT_FILE=$2
DURATION=${3:-60}

if [ "$(id -u)" -ne 0 ]; then
  echo "Этот скрипт должен запускаться с правами суперпользователя (sudo)"
  exit 1
fi

if ! command -v tcpdump &> /dev/null; then
  echo "Ошибка: утилита tcpdump не найдена. Пожалуйста, установите ее."
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_FILE")"

echo "Запуск захвата трафика на интерфейсе $INTERFACE. Захват будет продолжаться $DURATION секунд."
echo "Результат будет сохранен в файл: $OUTPUT_FILE"

timeout $DURATION tcpdump -i $INTERFACE -w $OUTPUT_FILE -v tcp

if [ $? -eq 124 ]; then
  echo "Захват трафика завершен по таймауту через $DURATION секунд."
elif [ $? -eq 0 ]; then
  echo "Захват трафика успешно завершен."
else
  echo "Ошибка при захвате трафика. Код ошибки: $?"
  exit 1
fi

echo "Захвачено пакетов:"
tcpdump -r $OUTPUT_FILE -qnn | wc -l

echo "Анализ SYN-пакетов (TCP handshake):"
tcpdump -r $OUTPUT_FILE -qnn "tcp[tcpflags] & (tcp-syn) != 0" | head -n 10

exit 0