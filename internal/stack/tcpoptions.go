package stack

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

type TCPOptions struct {
	WindowSize uint16

	TimestampsEnabled bool

	MSS uint16

	WindowScaleEnabled bool
	WindowScaleValue   uint8

	TTL uint8

	SACKEnabled bool

	OSType string
}

func GetTCPOptions(osType string, windowSize int, ttl int) (*TCPOptions, error) {
	switch osType {
	case "windows":
		return &TCPOptions{
			WindowSize:         uint16(windowSize),
			TimestampsEnabled:  false,
			MSS:                1460,
			WindowScaleEnabled: true,
			WindowScaleValue:   8,
			TTL:                uint8(ttl),
			SACKEnabled:        true,
			OSType:             "windows",
		}, nil

	case "macos":
		return &TCPOptions{
			WindowSize:         uint16(windowSize),
			TimestampsEnabled:  true,
			MSS:                1460,
			WindowScaleEnabled: true,
			WindowScaleValue:   6,
			TTL:                uint8(ttl),
			SACKEnabled:        true,
			OSType:             "macos",
		}, nil

	case "linux":
		return &TCPOptions{
			WindowSize:         uint16(windowSize),
			TimestampsEnabled:  true,
			MSS:                1460,
			WindowScaleEnabled: true,
			WindowScaleValue:   7,
			TTL:                uint8(ttl),
			SACKEnabled:        true,
			OSType:             "linux",
		}, nil

	default:
		return nil, fmt.Errorf("неизвестный тип ос для имитации: %s", osType)
	}
}

func ApplyTCPOptions(gs *GvisorStack, opts *TCPOptions) error {
	log.Printf("применение настроек tcp для имитации ос: %s", opts.OSType)

	if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.ip_default_ttl=%d", opts.TTL)); err != nil {
		return fmt.Errorf("не удалось установить ttl: %w", err)
	}

	if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.tcp_wmem='4096 %d %d'",
		opts.WindowSize, opts.WindowSize*2)); err != nil {
		return fmt.Errorf("не удалось установить tcp send buffer size: %w", err)
	}

	if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.tcp_rmem='4096 %d %d'",
		opts.WindowSize, opts.WindowSize*2)); err != nil {
		return fmt.Errorf("не удалось устоновить tcp receive buffer size: %w", err)
	}

	timestampsValue := "0"
	if opts.TimestampsEnabled {
		timestampsValue = "1"
	}
	if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.tcp_timestamps=%s", timestampsValue)); err != nil {
		return fmt.Errorf("не удалось установить tcp timestamps: %w", err)
	}

	windowScaleValue := "0"
	if opts.WindowScaleEnabled {
		windowScaleValue = "1"
		if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.tcp_window_scaling=%s", windowScaleValue)); err != nil {
			return fmt.Errorf("не удалось включить tcp window scaling: %w", err)
		}
	}

	routeCmd := fmt.Sprintf("ip route change default via $(ip route | grep default | awk '{print $3}') dev $(ip route | grep default | awk '{print $5}') advmss %d", opts.MSS)
	if err := executeCommand("bash", "-c", routeCmd); err != nil {
		log.Printf("предупреждение: не удалось установить mss: %v", err)
	}

	sackValue := "0"
	if opts.SACKEnabled {
		sackValue = "1"
	}
	if err := executeCommand("sysctl", "-w", fmt.Sprintf("net.ipv4.tcp_sack=%s", sackValue)); err != nil {
		return fmt.Errorf("не удалось установить tcp sack: %w", err)
	}

	log.Printf("настройки tcp успешно применены")
	return nil
}

func GetSocketOptions(socket *net.TCPConn) (map[string]interface{}, error) {
	socketOpts := make(map[string]interface{})

	file, err := socket.File()
	if err != nil {
		return nil, fmt.Errorf("не удалось получить файловый дескриптор: %w", err)
	}
	defer file.Close()

	if ttl, err := getSysctlValue("net.ipv4.ip_default_ttl"); err == nil {
		ttlInt, _ := strconv.Atoi(ttl)
		socketOpts["TTL"] = ttlInt
	}

	if windowSize, err := getSysctlValue("net.ipv4.tcp_rmem"); err == nil {
		parts := strings.Split(windowSize, "\t")
		if len(parts) >= 2 {
			socketOpts["WindowSize"] = parts[1]
		} else {
			socketOpts["WindowSize"] = windowSize
		}
	}

	if timestamps, err := getSysctlValue("net.ipv4.tcp_timestamps"); err == nil {
		tsInt, _ := strconv.Atoi(timestamps)
		socketOpts["TimestampsEnabled"] = (tsInt == 1)
	}

	if sack, err := getSysctlValue("net.ipv4.tcp_sack"); err == nil {
		sackInt, _ := strconv.Atoi(sack)
		socketOpts["SACKEnabled"] = (sackInt == 1)
	}

	return socketOpts, nil
}
