package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"custom-tcp-fingerprint/internal/analyzer"
	"custom-tcp-fingerprint/internal/network"
	"custom-tcp-fingerprint/internal/stack"
)

var (
	targetHost  = flag.String("host", "example.com", "Target host to connect to")
	targetPort  = flag.Int("port", 80, "Target port to connect to")
	tunName     = flag.String("tun", "tun0", "TUN interface name")
	localPort   = flag.Int("lport", 8080, "Local port to listen on")
	captureFile = flag.String("capture", "", "Capture traffic to file")
	windowSize  = flag.Int("window", 8192, "TCP Window Size")
	ttl         = flag.Int("ttl", 64, "IP Time to Live (TTL)")
	mtu         = flag.Int("mtu", 1500, "Maximum Transmission Unit (MTU)")
	fingerprint = flag.String("fp", "windows", "TCP fingerprint to imitate (windows, macos, linux)")
)

func main() {
	flag.Parse()

	log.Println("запуск инструмента кастомизации tcp-отпечатка")
	log.Printf("целевой хост: %s:%d", *targetHost, *targetPort)

	if os.Geteuid() != 0 {
		log.Fatal("эта программа должна запускатся с правами суперпользователя (sudo)")
	}

	tun, err := network.CreateTunInterface(*tunName, *mtu)
	if err != nil {
		log.Fatalf("не удалось создать tun-интерфейс: %v", err)
	}
	defer tun.Close()
	log.Printf("создан tun-интерфейс: %s", *tunName)

	if err := network.SetupIptablesRules(*tunName, *targetHost, *localPort); err != nil {
		log.Fatalf("не удалось настроить правила iptables: %v", err)
	}
	log.Println("правила iptables настроены успешно")

	if err := network.SetupRouting(*tunName, *targetHost); err != nil {
		log.Fatalf("не удалось настроить маршрутизацию: %v", err)
	}
	log.Println("маршрутизация настроена успешно")

	if *captureFile != "" {
		if dir := filepath.Dir(*captureFile); dir != "" {
			os.MkdirAll(dir, 0755)
		}

		go func() {
			if err := analyzer.CaptureTraffic(*tunName, *captureFile); err != nil {
				log.Printf("ошибка при захвате трафика: %v", err)
			}
		}()
		log.Printf("запущен захват трафика в фаил: %s", *captureFile)
	}

	s, err := stack.NewGvisorStack(*tunName, *mtu)
	if err != nil {
		log.Fatalf("не удалось создать сетевой стек: %v", err)
	}
	defer s.Close()

	beforeSettings := stack.GetCurrentFingerprint()
	log.Printf("текущие настройки tcp до изменений: %+v", beforeSettings)

	if err := stack.ConfigureTCPFingerprint(s, *fingerprint, *windowSize, *ttl); err != nil {
		log.Fatalf("не удалось настроить tcp-отпечаток: %v", err)
	}
	log.Printf("настроен tcp-отпечаток для имитации ос: %s", *fingerprint)

	afterSettings := stack.GetCurrentFingerprint()
	log.Printf("настройки tcp после изминений: %+v", afterSettings)

	if err := s.StartNetworking(*localPort, *targetHost, *targetPort); err != nil {
		log.Fatalf("не удалось запустить сетевой стек: %v", err)
	}
	log.Printf("запущен прокси на локальном порту %d", *localPort)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("\n======================================================\n")
	fmt.Printf("Сервис запущен и готов к использованию!\n")
	fmt.Printf("Для проверки подключитесь к localhost:%d\n", *localPort)
	fmt.Printf("Трафик будет перенаправлен на %s:%d с измененным TCP-отпечатком\n", *targetHost, *targetPort)
	fmt.Printf("Нажмите Ctrl+C для завершения работы\n")
	fmt.Printf("======================================================\n\n")

	<-sigCh
	log.Println("завершение работы...")

	time.Sleep(500 * time.Millisecond)

	if err := network.CleanupIptables(*tunName, *targetHost, *localPort); err != nil {
		log.Printf("ошибка при очистке правил iptables: %v", err)
	} else {
		log.Println("правила iptables очищены")
	}

	if err := network.CleanupRouting(*tunName, *targetHost); err != nil {
		log.Printf("ошыбка при очистке маршрутизации: %v", err)
	} else {
		log.Println("маршрутизация очищена")
	}

	fmt.Println("Все ресурсы освобождены, программа завершена")
}
