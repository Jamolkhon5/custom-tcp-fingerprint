package network

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	MARK_VALUE = "0x1337"
)

func SetupIptablesRules(tunName, targetHost string, localPort int) error {
	chains := []struct {
		table string
		chain string
	}{
		{"filter", "INPUT"},
		{"filter", "OUTPUT"},
		{"filter", "FORWARD"},
		{"nat", "PREROUTING"},
		{"nat", "POSTROUTING"},
		{"mangle", "PREROUTING"},
		{"mangle", "OUTPUT"},
	}

	for _, c := range chains {
		if err := checkAndCreateChain(c.table, c.chain); err != nil {
			return fmt.Errorf("failed to check/create chain %s in table %s: %w", c.chain, c.table, err)
		}
	}

	rules := [][]string{
		{"filter", "-A", "INPUT", "-p", "tcp", "--dport", fmt.Sprintf("%d", localPort), "-j", "ACCEPT"},

		{"mangle", "-A", "OUTPUT", "-p", "tcp", "--sport", fmt.Sprintf("%d", localPort), "-j", "MARK", "--set-mark", MARK_VALUE},

		{"mangle", "-A", "OUTPUT", "-p", "tcp", "-d", targetHost, "-j", "MARK", "--set-mark", MARK_VALUE},

		{"filter", "-A", "FORWARD", "-i", "lo", "-o", tunName, "-m", "mark", "--mark", MARK_VALUE, "-j", "ACCEPT"},
		{"filter", "-A", "FORWARD", "-i", tunName, "-o", "lo", "-m", "mark", "--mark", MARK_VALUE, "-j", "ACCEPT"},

		{"nat", "-A", "POSTROUTING", "-o", tunName, "-j", "MASQUERADE"},
	}

	for _, rule := range rules {
		cmd := exec.Command("iptables", append([]string{"-t", rule[0]}, rule[1:]...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to apply iptables rule: %s, error: %w, output: %s",
				strings.Join(rule, " "), err, string(output))
		}
		log.Printf("применено правило iptables: %s", strings.Join(rule, " "))
	}

	if err := enableIPForwarding(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	return nil
}

func CleanupIptables(tunName, targetHost string, localPort int) error {
	rules := [][]string{
		{"nat", "-D", "POSTROUTING", "-o", tunName, "-j", "MASQUERADE"},

		{"filter", "-D", "FORWARD", "-i", tunName, "-o", "lo", "-m", "mark", "--mark", MARK_VALUE, "-j", "ACCEPT"},
		{"filter", "-D", "FORWARD", "-i", "lo", "-o", tunName, "-m", "mark", "--mark", MARK_VALUE, "-j", "ACCEPT"},

		{"mangle", "-D", "OUTPUT", "-p", "tcp", "-d", targetHost, "-j", "MARK", "--set-mark", MARK_VALUE},
		{"mangle", "-D", "OUTPUT", "-p", "tcp", "--sport", fmt.Sprintf("%d", localPort), "-j", "MARK", "--set-mark", MARK_VALUE},

		{"filter", "-D", "INPUT", "-p", "tcp", "--dport", fmt.Sprintf("%d", localPort), "-j", "ACCEPT"},
	}

	for _, rule := range rules {
		cmd := exec.Command("iptables", append([]string{"-t", rule[0]}, rule[1:]...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("предупреждение: не удалось удалить правило iptables: %s, ошибка: %v, вывод: %s",
				strings.Join(rule, " "), err, string(output))
		} else {
			log.Printf("удалено правило iptables: %s", strings.Join(rule, " "))
		}
	}

	return nil
}

func checkAndCreateChain(table, chain string) error {
	cmd := exec.Command("iptables", "-t", table, "-L", chain)
	if err := cmd.Run(); err != nil {
		createCmd := exec.Command("iptables", "-t", table, "-N", chain)
		if output, err := createCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create chain: %s, output: %s", err, string(output))
		}
	}
	return nil
}

func enableIPForwarding() error {
	cmd := exec.Command("cat", "/proc/sys/net/ipv4/ip_forward")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		log.Println("ip forwarding уже включон")
		return nil
	}

	cmd = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("предупреждение: не удалось включить ip forwarding через sysctl: %v", err)

		err = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644)
		if err != nil {
			log.Printf("предупреждение: не удалось включить ip forwarding через запись в файл: %v", err)
			log.Printf("продолжаем без ip forwarding, некоторые функции могут не работать")
			return nil
		}
	}

	log.Println("ip forwarding успешно включен")
	return nil
}
