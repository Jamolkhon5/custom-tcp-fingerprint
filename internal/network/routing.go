package network

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
)

func SetupRouting(tunName, targetHost string) error {
	targetIPs, err := net.LookupIP(targetHost)
	if err != nil {
		return fmt.Errorf("failed to resolve target host: %w", err)
	}

	var targetIP net.IP
	for _, ip := range targetIPs {
		if ip.To4() != nil {
			targetIP = ip
			break
		}
	}

	if targetIP == nil {
		return fmt.Errorf("no IPv4 address found for target host: %s", targetHost)
	}

	log.Printf("целевой ip: %s", targetIP.String())

	cmd := exec.Command("ip", "addr", "add", "10.0.0.1/24", "dev", tunName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to assign IP to TUN interface: %s, output: %s", err, string(output))
	}

	cmds := [][]string{
		{"ip", "rule", "add", "fwmark", MARK_VALUE, "table", "100"},

		{"ip", "route", "add", targetIP.String(), "dev", tunName, "table", "100"},

		{"ip", "route", "add", "default", "via", "10.0.0.1", "dev", tunName, "table", "100"},
	}

	for _, cmd := range cmds {
		command := exec.Command(cmd[0], cmd[1:]...)
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to run command '%s': %s, output: %s",
				strings.Join(cmd, " "), err, string(output))
		}
		log.Printf("применена команда маршрутизации: %s", strings.Join(cmd, " "))
	}

	return nil
}

func CleanupRouting(tunName, targetHost string) error {
	targetIPs, err := net.LookupIP(targetHost)
	if err != nil {
		log.Printf("предупреждение: не удалось разрешить целевой хост: %v", err)
		return nil
	}

	var targetIP net.IP
	for _, ip := range targetIPs {
		if ip.To4() != nil {
			targetIP = ip
			break
		}
	}

	if targetIP == nil {
		log.Printf("предупреждение: не наиден ipv4 адрес для целевого хоста: %s", targetHost)
		return nil
	}

	cmds := [][]string{
		{"ip", "route", "del", "default", "via", "10.0.0.1", "dev", tunName, "table", "100"},

		{"ip", "route", "del", targetIP.String(), "dev", tunName, "table", "100"},

		{"ip", "rule", "del", "fwmark", MARK_VALUE, "table", "100"},
	}

	for _, cmd := range cmds {
		command := exec.Command(cmd[0], cmd[1:]...)
		if output, err := command.CombinedOutput(); err != nil {
			log.Printf("предупреждение: не удалось выполнить команду '%s': %s, вывод: %s",
				strings.Join(cmd, " "), err, string(output))
		} else {
			log.Printf("удалена команда маршрутизации: %s", strings.Join(cmd, " "))
		}
	}

	cmd := exec.Command("ip", "addr", "del", "10.0.0.1/24", "dev", tunName)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("предупреждение: не удалось удалить ip с tun интерфейса: %s, вывод: %s",
			err, string(output))
	}

	return nil
}
