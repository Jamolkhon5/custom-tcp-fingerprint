package stack

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"time"
)

type GvisorStack struct {
	tunName     string
	mtu         int
	targetHost  string
	targetPort  int
	localPort   int
	isConnected bool
}

func NewGvisorStack(tunName string, mtu int) (*GvisorStack, error) {
	cmd := exec.Command("ip", "link", "show", tunName)
	if err := cmd.Run(); err == nil {
		exec.Command("ip", "tuntap", "del", "dev", tunName, "mode", "tun").Run()
	}

	cmd = exec.Command("ip", "tuntap", "add", "dev", tunName, "mode", "tun")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ошыбка создания tun интерфейса: %s, вывод: %s", err, string(out))
	}

	cmd = exec.Command("ip", "link", "set", "dev", tunName, "mtu", fmt.Sprintf("%d", mtu))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ошибка установки mtu: %s, вывод: %s", err, string(out))
	}

	cmd = exec.Command("ip", "link", "set", "dev", tunName, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ошибка активации интерфейса: %s, вывод: %s", err, string(out))
	}

	log.Printf("создан и настроен tun интерфейс: %s с mtu %d", tunName, mtu)

	return &GvisorStack{
		tunName:     tunName,
		mtu:         mtu,
		isConnected: false,
	}, nil
}

func (g *GvisorStack) StartNetworking(localPort int, targetHost string, targetPort int) error {
	g.localPort = localPort
	g.targetHost = targetHost
	g.targetPort = targetPort

	addrs, err := net.LookupHost(targetHost)
	if err != nil {
		log.Printf("предупреждение: не удалось выполнить dns-запрос для %s: %v", targetHost, err)
		log.Printf("продолжаем работу, но соединение может быть невозможно")
	} else {
		log.Printf("целевой хост %s разрешается в ip-адреса: %v", targetHost, addrs)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return fmt.Errorf("не удалось запустить слушающий сокет: %w", err)
	}

	log.Printf("запущен прокси на порту %d, перенаправление на %s:%d", localPort, targetHost, targetPort)
	g.isConnected = true

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("ошибка при принятии соединения: %v", err)
				break
			}
			go g.handleConnection(conn)
		}
	}()

	return nil
}

func (g *GvisorStack) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	targetAddr := fmt.Sprintf("%s:%d", g.targetHost, g.targetPort)
	log.Printf("установка соединения с %s", targetAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var dialer net.Dialer
	serverConn, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		log.Printf("ошибка при соединении с целевым хостом: %v", err)
		return
	}
	defer serverConn.Close()

	errChan := make(chan error, 2)

	go func() {
		_, err := io.Copy(serverConn, clientConn)
		errChan <- err
	}()

	go func() {
		_, err := io.Copy(clientConn, serverConn)
		errChan <- err
	}()

	err = <-errChan
	if err != nil {
		log.Printf("соединение прервоно: %v", err)
	}
}

func (g *GvisorStack) Close() {
	if g.isConnected {
		cmd := exec.Command("ip", "tuntap", "del", "dev", g.tunName, "mode", "tun")
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("ошибка при удалении tun интерфейса: %s, вывод: %s", err, string(out))
		}
		g.isConnected = false
	}
}
