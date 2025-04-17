package analyzer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func CaptureTraffic(interfaceName, outputFile string) error {
	time.Sleep(2 * time.Second)

	dir := filepath.Dir(outputFile)
	if dir != "" {
		os.MkdirAll(dir, 0755)
	}

	cmd := exec.Command("ip", "link", "show", interfaceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interface %s does not exist: %w", interfaceName, err)
	}

	args := []string{
		"-i", interfaceName,
		"-w", outputFile,
		"-v",
		"tcp",
	}

	log.Printf("начинаем захват трафика на интерфейсе %s, сохраняем в %s", interfaceName, outputFile)

	cmd = exec.Command("tcpdump", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tcpdump: %w", err)
	}

	pid := cmd.Process.Pid
	log.Printf("tcpdump запущен с пид %d", pid)

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("tcpdump завершился с ошыбкой: %v", err)
		} else {
			log.Printf("tcpdump завершился успешно")
		}
	}()

	return nil
}

func CaptureTCPHandshake(interfaceName, outputFile string, durationSeconds int) error {
	args := []string{
		"-i", interfaceName,
		"-w", outputFile,
		"-v",
		"tcp[tcpflags] & (tcp-syn|tcp-ack) != 0",
		"-c", "10",
	}

	log.Printf("начинаем захват tcp-хендшейка на интерфейсе %s, сохраняем в %s", interfaceName, outputFile)

	cmd := exec.Command("tcpdump", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tcpdump for handshake: %w", err)
	}

	timer := time.AfterFunc(time.Duration(durationSeconds)*time.Second, func() {
		if cmd.Process != nil {
			log.Printf("останавливаем tcpdump после %d секунд", durationSeconds)
			cmd.Process.Signal(os.Interrupt)
		}
	})
	defer timer.Stop()

	if err := cmd.Wait(); err != nil {
		if cmd.ProcessState.ExitCode() == 1 {
			log.Printf("захват tcp-хендшейка завершен (таймаут)")
			return nil
		}
		return fmt.Errorf("tcpdump handshake capture failed: %w", err)
	}

	log.Printf("захват tcp-хендшейка завершен успешно")
	return nil
}

func AnalyzePcapFile(pcapFile string) (string, error) {
	if _, err := os.Stat(pcapFile); os.IsNotExist(err) {
		return "", fmt.Errorf("pcap file does not exist: %s", pcapFile)
	}

	args := []string{
		"-r", pcapFile,
		"-Y", "tcp.flags.syn==1 && tcp.flags.ack==0",
		"-T", "fields",
		"-e", "ip.src",
		"-e", "ip.dst",
		"-e", "tcp.window_size",
		"-e", "ip.ttl",
		"-e", "tcp.options",
	}

	cmd := exec.Command("tshark", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to analyze pcap file: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
