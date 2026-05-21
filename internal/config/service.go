package config

import (
	"fmt"
	"os"
	"os/exec"
)

func InstallRootService() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get exe path: %v", err)
	}

	finalPath := BinaryInstallPath
	if exePath != finalPath {
		input, err := os.ReadFile(exePath)
		if err != nil {
			return err
		}
		err = os.WriteFile(finalPath, input, 0755)
		if err != nil {
			return fmt.Errorf("failed to move binary: %v", err)
		}
	}

	// создает unit
	serviceContent := fmt.Sprintf(`[Unit]
Description=%s service
After=network.target

[Service]
Type=simple
User=root
ExecStart=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, ServiceName, finalPath)

	// запись в systemd
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", ServiceName)
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write service file: %v", err)
	}

	// активэйшн
	serviceFileName := fmt.Sprintf("%s.service", ServiceName)
	commands := [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", serviceFileName},
		{"systemctl", "start", serviceFileName},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl error %v: %v, output: %s", cmdArgs, err, string(out))
		}
	}

	return nil
}
