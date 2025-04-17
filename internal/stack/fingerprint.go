package stack

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type SystemTCPOptions struct {
	WindowSize        uint16
	TimestampsEnabled bool
	MSS               uint16
	WindowScaleValue  uint8
	TTL               uint8
	OSType            string
}

func ConfigureTCPFingerprint(gs *GvisorStack, osType string, windowSize int, ttl int) error {
	log.Printf("настройка tcp-отпечатка под %s с размером окна %d и ttl %d",
		osType, windowSize, ttl)

	opts, err := GetSystemTCPOptions(osType, windowSize, ttl)
	if err != nil {
		return fmt.Errorf("не удалось получить tcp опции: %w", err)
	}

	if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.ip_default_ttl=%d", opts.TTL)); err != nil {
		log.Printf("предупреждение: не удалось установить ttl: %v", err)
	}

	tcpRmemCmd := fmt.Sprintf("net.ipv4.tcp_rmem=\"4096 %d 6291456\"", opts.WindowSize)
	if err := executeCommand("sysctl", "-w", tcpRmemCmd); err != nil {
		log.Printf("предупреждение: не удалось устоновить размер tcp окна: %v", err)
	}

	timestampsValue := "0"
	if opts.TimestampsEnabled {
		timestampsValue = "1"
	}
	if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.tcp_timestamps=%s", timestampsValue)); err != nil {
		log.Printf("предупреждение: не удалось установить tcp timestamps: %v", err)
	}

	if err := executeCommand("ip", "link", "set", "dev", gs.tunName, "mtu", fmt.Sprintf("%d", opts.MSS)); err != nil {
		log.Printf("предупреждение: не удалось установить mtu для интерфейса: %v", err)
	}

	log.Printf("tcp-отпечаток настроен успешно")
	return nil
}

func GetSystemTCPOptions(osType string, windowSize int, ttl int) (*SystemTCPOptions, error) {
	switch osType {
	case "windows":
		return &SystemTCPOptions{
			WindowSize:        uint16(windowSize),
			TimestampsEnabled: false,
			MSS:               1460,
			WindowScaleValue:  8,
			TTL:               uint8(ttl),
			OSType:            "windows",
		}, nil

	case "macos":
		return &SystemTCPOptions{
			WindowSize:        uint16(windowSize),
			TimestampsEnabled: true,
			MSS:               1460,
			WindowScaleValue:  6,
			TTL:               uint8(ttl),
			OSType:            "macos",
		}, nil

	case "linux":
		return &SystemTCPOptions{
			WindowSize:        uint16(windowSize),
			TimestampsEnabled: true,
			MSS:               1460,
			WindowScaleValue:  7,
			TTL:               uint8(ttl),
			OSType:            "linux",
		}, nil

	default:
		return nil, fmt.Errorf("неизвестный тип отпечатка ос: %s", osType)
	}
}

func GetCurrentFingerprint() map[string]interface{} {
	fingerprint := make(map[string]interface{})

	if ttl, err := getSysctlValue("net.ipv4.ip_default_ttl"); err == nil {
		fingerprint["TTL"] = ttl
	}

	if windowSize, err := getSysctlValue("net.ipv4.tcp_rmem"); err == nil {
		fingerprint["WindowSize"] = windowSize
	}

	if timestamps, err := getSysctlValue("net.ipv4.tcp_timestamps"); err == nil {
		fingerprint["Timestamps"] = timestamps
	}

	cmd := exec.Command("ip", "route", "show")
	if out, err := cmd.CombinedOutput(); err == nil {
		routes := string(out)
		if strings.Contains(routes, "advmss") {
			parts := strings.Split(routes, "advmss")
			if len(parts) > 1 {
				mssParts := strings.Split(strings.TrimSpace(parts[1]), " ")
				if len(mssParts) > 0 {
					fingerprint["MSS"] = mssParts[0]
				}
			}
		}
	}

	if sack, err := getSysctlValue("net.ipv4.tcp_sack"); err == nil {
		fingerprint["SACK"] = sack
	}

	return fingerprint
}

func getSysctlValue(param string) (string, error) {
	cmd := exec.Command("sysctl", "-n", param)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func executeCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ошибка выполнения команды '%s %s': %w, вывод: %s",
			command, strings.Join(args, " "), err, string(out))
	}
	return nil
}
