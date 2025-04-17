package network

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type TUNInterface struct {
	name   string
	file   *os.File
	active bool
}

func CreateTunInterface(tunName string, mtu int) (*TUNInterface, error) {
	cmd := exec.Command("ip", "tuntap", "add", "dev", tunName, "mode", "tun")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("не удалось создать tun интерфейс: %s, вывод: %s", err, string(output))
	}

	cmd = exec.Command("ip", "link", "set", "dev", tunName, "mtu", strconv.Itoa(mtu))
	if output, err := cmd.CombinedOutput(); err != nil {
		exec.Command("ip", "tuntap", "del", "dev", tunName, "mode", "tun").Run()
		return nil, fmt.Errorf("не удалось установить mtu: %s, вывод: %s", err, string(output))
	}

	cmd = exec.Command("ip", "link", "set", "dev", tunName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		exec.Command("ip", "tuntap", "del", "dev", tunName, "mode", "tun").Run()
		return nil, fmt.Errorf("не удалось поднять интерфейс: %s, вывод: %s", err, string(output))
	}

	path := filepath.Join("/dev", tunName)
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		file, err = os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
		if err != nil {
			exec.Command("ip", "tuntap", "del", "dev", tunName, "mode", "tun").Run()
			return nil, fmt.Errorf("не удалось открыть файловый дескриптор: %w", err)
		}
	}

	return &TUNInterface{
		name:   tunName,
		file:   file,
		active: true,
	}, nil
}

func (t *TUNInterface) Name() string {
	return t.name
}

func (t *TUNInterface) File() *os.File {
	return t.file
}

func (t *TUNInterface) Close() error {
	if !t.active {
		return nil
	}

	if err := t.file.Close(); err != nil {
		return fmt.Errorf("не удалось закрыть файловый дескриптор: %w", err)
	}

	cmd := exec.Command("ip", "tuntap", "del", "dev", t.name, "mode", "tun")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("не удалось удалить tun интерфейс: %s, вывод: %s", err, string(output))
	}

	t.active = false
	return nil
}
