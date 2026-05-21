package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// стили
var (
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true).MarginBottom(1)
	fieldStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
)

type Preset struct {
	Name    string
	Redis   string
	Pass    string
	Github  string
	SrvName string
}

// TODO
var presets = []Preset{
	// {"Production", "123.123.123.123:6379", "prod_pass_123", "https://raw.githubusercontent.com/...", ""},
	{"Testing", "127.0.0.1:6379", "test_pass_123", "https://raw.githubusercontent.com/...", "test_cluster"},
}

type model struct {
	inputs  []textinput.Model
	cursor  int
	focused bool
	status  string
	outPath string
}

func initialModel() model {
	inputs := make([]textinput.Model, 8)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Redis Address"
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Redis Password"

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Github Config URL"

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "Service Name"

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "Remote Install Path"

	inputs[5] = textinput.New()
	inputs[5].Placeholder = "Heartbeat Interval (e.g. 30s, 1m)"
	inputs[5].SetValue("30s")

	inputs[6] = textinput.New()
	inputs[6].Placeholder = "Client Response Timeout (e.g. 8s, 15s)"
	inputs[6].SetValue("8s")

	inputs[7] = textinput.New()
	inputs[7].Placeholder = "Local Build Output Dir"
	inputs[7].SetValue("./build")

	return model{
		inputs:  inputs,
		cursor:  0,
		focused: true,
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "tab", "shift+tab":
			// переключение между полями
			m.inputs[m.cursor].Blur()
			if msg.String() == "tab" {
				m.cursor = (m.cursor + 1) % len(m.inputs)
			} else {
				m.cursor = (m.cursor - 1 + len(m.inputs)) % len(m.inputs)
			}
			m.inputs[m.cursor].Focus()
			return m, nil

		case "enter":
			if m.cursor == len(m.inputs)-1 {
				// ЗАПУСК СБОРКИ
				err := m.buildProject()
				if err != nil {
					m.status = errorStyle.Render(fmt.Sprintf("[x] Error: %v", err))
				} else {
					m.status = successStyle.Render("[v] Build Successful! Files in build folder.")
				}
			} else {
				// переходим к следующему полю
				m.inputs[m.cursor].Blur()
				m.cursor = (m.cursor + 1) % len(m.inputs)
				m.inputs[m.cursor].Focus()
			}
		}
	}

	// обновляем состояние активного ввода
	m.inputs[m.cursor], cmd = m.inputs[m.cursor].Update(msg)
	return m, cmd
}

func (m model) View() string {
	s := titleStyle.Render("== CLUSTER BUILDER v1.0 ==\n")
	s += "Use TAB to navigate, ENTER to build\n\n"

	for i := 0; i < len(m.inputs); i++ {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		s += fmt.Sprintf("%s %s\n", cursor, m.inputs[i].View())
	}

	s += "\n" + m.status + "\n"
	return s
}

func (m model) buildProject() error {
	vals := []string{}
	for _, input := range m.inputs {
		vals = append(vals, input.Value())
	}

	redisAddr := vals[0]
	redisPass := vals[1]
	githubURL := vals[2]
	srvName := vals[3]
	remotePath := vals[4]
	hbInterval := vals[5]
	resTimeout := vals[6]
	outDir := vals[7]

	if redisAddr == "" || srvName == "" {
		return fmt.Errorf("redis address and service name are required")
	}

	os.MkdirAll(outDir, 0755)

	// список ldflags
	flags := fmt.Sprintf("-X 'cluster/internal/config.DefaultRedisAddr=%s' "+
		"-X 'cluster/internal/config.DefaultRedisPass=%s' "+
		"-X 'cluster/internal/config.DefaultGithubURL=%s' "+
		"-X 'cluster/internal/config.ServiceName=%s' "+
		"-X 'cluster/internal/config.BinaryInstallPath=%s' "+
		"-X 'cluster/internal/config.HeartbeatInterval=%s' "+
		"-X 'cluster/internal/config.ResponseTimeout=%s'",
		redisAddr, redisPass, githubURL, srvName, remotePath, hbInterval, resTimeout)

	// сборка агента
	agentCmd := exec.Command("go", "build", "-ldflags", flags, "-o", outDir+"/agent", "./cmd/agent/main.go")
	if out, err := agentCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("agent build failed: %v\n%s", err, string(out))
	}

	// сборка клиента
	clientCmd := exec.Command("go", "build", "-ldflags", flags, "-o", outDir+"/client", "./cmd/client/main.go")
	if out, err := clientCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("client build failed: %v\n%s", err, string(out))
	}

	return nil
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
