package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cluster/internal/config"
	"cluster/internal/redisclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	if cfg.RedisAddr == "" {
		log.Fatal("Адрес Redis не задан в конфигурационном файле.")
	}

	manager := redisclient.NewManager(ctx, cfg.RedisAddr, cfg.RedisPass)

	fmt.Println("=== Панель управления кластером ===")
	fmt.Printf("Подключение к Redis: %s\n", cfg.RedisAddr)
	fmt.Println("Введите 'help' для списка доступных команд.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nmanager> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.SplitN(input, " ", 3)
		cmd := parts[0]

		switch cmd {
		case "help":
			printHelp()
		case "list":
			listDevices(manager)
		case "exec":
			if len(parts) < 3 {
				fmt.Println("Использование: exec <device_id> <command>")
				continue
			}
			executeOnDevice(manager, parts[1], parts[2])
		case "exec-all":
			if len(parts) < 2 {
				fmt.Println("Использование: exec-all <command>")
				continue
			}
			fullCmd := strings.Join(parts[1:], " ")
			executeOnAll(manager, fullCmd)
		case "set-github":
			if len(parts) < 3 {
				fmt.Println("Использование: set-github <device_id> <url>")
				continue
			}
			executeOnDevice(manager, parts[1], "SET_GITHUB_URL:"+parts[2])
		case "prune":
			count, err := manager.PruneDevices()
			if err != nil {
				fmt.Printf("Ошибка при очистке: %v\n", err)
			} else {
				fmt.Printf("[v] Очистка завершена. Удалено мертвых устройств: %d\n", count)
			}
		case "uninstall-all":
			uninstallAll(manager, scanner)
		case "exit", "quit":
			fmt.Println("Завершение работы.")
			return
		default:
			fmt.Printf("Неизвестная команда: %s. Введите 'help' для списка.\n", cmd)
		}
	}
}

func printHelp() {
	fmt.Println("\nДоступные команды:")
	fmt.Println("  list                           - Показать список всех устройств и их статус")
	fmt.Println("  exec <device_id> <command>     - Выполнить команду на конкретном устройстве")
	fmt.Println("  exec-all <command>             - Выполнить команду на всех устройствах")
	fmt.Println("  set-github <device_id> <url>   - Обновить URL конфигурации GitHub на устройстве")
	fmt.Println("  prune                          - Удалить из списка все Offline устройства")
	fmt.Println("  uninstall-all                  - ПОЛНОЕ УДАЛЕНИЕ агента со всех машин (для root)")
	fmt.Println("  exit / quit                    - Выйти из программы")
}

// устройства в кластере
func listDevices(mgr *redisclient.Manager) {
	devices, err := mgr.GetDevices()
	if err != nil {
		fmt.Printf("Ошибка получения списка устройств: %v\n", err)
		return
	}

	if len(devices) == 0 {
		fmt.Println("Устройства не найдены в базе данных Redis.")
		return
	}

	fmt.Printf("\n%-36s | %-15s | %-8s | %-7s | %s\n", "DEVICE ID", "HOSTNAME", "STATUS", "IS ROOT", "LAST SEEN")
	fmt.Println(strings.Repeat("-", 95))
	for _, dev := range devices {
		lastSeenTime := time.Unix(dev.LastSeen, 0).Format("2006-01-02 15:04:05")
		statusStr := "Offline"
		if dev.Status == "Online" {
			statusStr = "[o] Online"
		} else {
			statusStr = "[x] Offline"
		}
		fmt.Printf("%-36s | %-15s | %-8s | %-7t | %s\n", dev.ID, dev.Hostname, statusStr, dev.IsRoot, lastSeenTime)
	}
}

// полное удаление агентов со всех машин
func uninstallAll(mgr *redisclient.Manager, scanner *bufio.Scanner) {
	fmt.Println("\n[!]  ВНИМАНИЕ: Эта команда удалит агент, сервис и все конфигурации со ВСЕХ машин!")

	fmt.Print("Вы уверены? (y/n): ")
	if !scanner.Scan() || strings.ToLower(scanner.Text()) != "y" {
		fmt.Println("Отмена операции.")
		return
	}

	fmt.Print("[x] Данные будут полностью стерты. Продолжить? (y/n): ")
	if !scanner.Scan() || strings.ToLower(scanner.Text()) != "y" {
		fmt.Println("Отмена операции.")
		return
	}

	uninstallCmd := fmt.Sprintf(
		"sudo systemctl stop %s && sudo systemctl disable %s && sudo rm -f /etc/systemd/system/%s.service && sudo rm -f %s && sudo rm -rf /etc/cluster && sudo systemctl daemon-reload && sudo systemctl reset-failed",
		config.ServiceName, config.ServiceName, config.ServiceName, config.BinaryInstallPath,
	)

	fmt.Println("[v] Рассылка команды удаления на все устройства...")
	executeOnAll(mgr, uninstallCmd)
}

// выполнит на одном устройстве
func executeOnDevice(mgr *redisclient.Manager, deviceID string, command string) {
	ch, pubsub, err := mgr.ListenToResults(deviceID)
	if err != nil {
		fmt.Printf("Ошибка подписки на результаты: %v\n", err)
		return
	}
	defer pubsub.Close()

	err = mgr.SendCommand(deviceID, command)
	if err != nil {
		fmt.Printf("Ошибка отправки команды: %v\n", err)
		return
	}

	fmt.Println("Команда отправлена. Ожидание ответа...")

	timeoutDuration, _ := time.ParseDuration(config.ResponseTimeout)

	select {
	case msg := <-ch:
		fmt.Printf("\n[Ответ от %s]:\n%s\n", deviceID, msg.Payload)
	case <-time.After(timeoutDuration):
		fmt.Printf("Превышено время ожидания (%v) от устройства.\n", timeoutDuration)
	}
}

// Выполнить на всех
func executeOnAll(mgr *redisclient.Manager, command string) {
	ch, pubsub, err := mgr.ListenToResults("all")
	if err != nil {
		fmt.Printf("Ошибка подписки на результаты: %v\n", err)
		return
	}
	defer pubsub.Close()

	err = mgr.SendCommandAll(command)
	if err != nil {
		fmt.Printf("Ошибка рассылки команды: %v\n", err)
		return
	}

	timeoutDuration, _ := time.ParseDuration(config.ResponseTimeout)
	fmt.Printf("Команда отправлена всем. Сбор ответов в течение %v...\n", timeoutDuration)

	timeout := time.After(timeoutDuration)
	for {
		select {
		case msg := <-ch:
			parts := strings.Split(msg.Channel, ":")
			senderID := "unknown"
			if len(parts) > 1 {
				senderID = parts[1]
			}
			fmt.Printf("\n[Ответ от %s]:\n%s\n", senderID, msg.Payload)
		case <-timeout:
			fmt.Println("\nСбор ответов завершен.")
			return
		}
	}
}
