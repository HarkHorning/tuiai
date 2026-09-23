package tui

import (
	"fmt"
	"strings"

	"tuiai/internal/ai"
	"tuiai/internal/fs"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AppState int

const (
	StateChat AppState = iota
	StateReview
	StateHelp
)

type Model struct {
	state       AppState
	sandbox     *fs.Sandbox
	aiClient    *ai.Client
	textInput   textinput.Model
	spinner     spinner.Model
	thinking    bool
	chatLog     []string
	width       int
	height      int

	// Pending file action for review
	pendingAction   string
	pendingFilename string
	pendingContent  string
}

func InitialModel() (*Model, error) {
	sandbox, err := fs.NewSandbox()
	if err != nil {
		return nil, err
	}

	ti := textinput.New()
	ti.Placeholder = "Type your message or /help..."
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 60

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	return &Model{
		state:     StateChat,
		sandbox:   sandbox,
		aiClient:  ai.NewClient("tuiai-model"),
		textInput: ti,
		spinner:   s,
		chatLog: []string{
			"=== TUIAI Book Assistant initialized ===",
			"Type /help for available commands or start chatting with your notes.",
		},
	}, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.state == StateReview || m.state == StateHelp {
				m.state = StateChat
				return m, nil
			}
			return m, tea.Quit
		}

		// Handle states
		switch m.state {
		case StateHelp:
			if msg.String() == "enter" || msg.String() == " " {
				m.state = StateChat
			}
			return m, nil

		case StateReview:
			switch strings.ToLower(msg.String()) {
			case "y", "Y":
				// Execute pending action
				var err error
				if m.pendingAction == "write" {
					err = m.sandbox.WriteFile(m.pendingFilename, m.pendingContent)
				} else if m.pendingAction == "delete" {
					err = m.sandbox.DeleteFile(m.pendingFilename)
				}
				if err != nil {
					m.chatLog = append(m.chatLog, fmt.Sprintf("[Error executing %s]: %v", m.pendingAction, err))
				} else {
					m.chatLog = append(m.chatLog, fmt.Sprintf("[Success]: %s applied to %s", m.pendingAction, m.pendingFilename))
				}
				m.state = StateChat
				return m, nil
			case "n", "N", "esc":
				m.chatLog = append(m.chatLog, fmt.Sprintf("[Cancelled]: %s on %s was denied.", m.pendingAction, m.pendingFilename))
				m.state = StateChat
				return m, nil
			}
			return m, nil

		case StateChat:
			if msg.String() == "enter" {
				val := strings.TrimSpace(m.textInput.Value())
				if val == "" {
					return m, nil
				}
				m.textInput.Reset()

				// Handle slash commands
				if strings.HasPrefix(val, "/") {
					switch val {
					case "/exit":
						return m, tea.Quit
					case "/read-only":
						m.sandbox.ReadOnly = !m.sandbox.ReadOnly
						m.chatLog = append(m.chatLog, fmt.Sprintf("[System] ReadOnly mode set to: %v", m.sandbox.ReadOnly))
						return m, nil
					case "/disable-delete":
						m.sandbox.DisableDelete = !m.sandbox.DisableDelete
						m.chatLog = append(m.chatLog, fmt.Sprintf("[System] DisableDelete mode set to: %v", m.sandbox.DisableDelete))
						return m, nil
					case "/help":
						m.state = StateHelp
						return m, nil
					default:
						m.chatLog = append(m.chatLog, fmt.Sprintf("[System] Unknown command: %s. Type /help", val))
						return m, nil
					}
				}

				// User chat message
				m.chatLog = append(m.chatLog, "You: "+val)
				m.aiClient.AddMessage("user", val)
				m.thinking = true

				// Trigger AI call
				return m, func() tea.Msg {
					resp, err := m.aiClient.SendChat()
					return aiResponseMsg{resp: resp, err: err}
				}
			}
		}

	case aiResponseMsg:
		m.thinking = false
		if msg.err != nil {
			m.chatLog = append(m.chatLog, fmt.Sprintf("[AI Error]: %v", msg.err))
			return m, nil
		}

		// Handle tool calls if requested by AI
		if len(msg.resp.ToolCalls) > 0 {
			tc := msg.resp.ToolCalls[0]
			name := tc.Function.Name
			args := tc.Function.Arguments

			switch name {
			case "list_files":
				files, err := m.sandbox.ListFiles()
				result := ""
				if err != nil {
					result = fmt.Sprintf("Error listing files: %v", err)
				} else {
					result = "Files found: " + strings.Join(files, ", ")
				}
				m.aiClient.AddMessage("assistant", "[Tool Result: "+result+"]")
				m.chatLog = append(m.chatLog, "AI: [Listed workspace files]")

			case "read_file":
				fname, _ := args["filename"].(string)
				content, err := m.sandbox.ReadFile(fname)
				result := ""
				if err != nil {
					result = fmt.Sprintf("Error reading %s: %v", fname, err)
				} else {
					result = content
				}
				m.aiClient.AddMessage("assistant", "[Read File "+fname+"]")
				m.chatLog = append(m.chatLog, fmt.Sprintf("AI: [Read file %s]", fname))

			case "write_file":
				fname, _ := args["filename"].(string)
				content, _ := args["content"].(string)
				m.pendingAction = "write"
				m.pendingFilename = fname
				m.pendingContent = content
				m.state = StateReview
				return m, nil

			case "delete_file":
				fname, _ := args["filename"].(string)
				m.pendingAction = "delete"
				m.pendingFilename = fname
				m.pendingContent = ""
				m.state = StateReview
				return m, nil
			}
		} else if msg.resp.Content != "" {
			m.aiClient.AddMessage("assistant", msg.resp.Content)
			m.chatLog = append(m.chatLog, "AI: "+msg.resp.Content)
		}
		return m, nil

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

type aiResponseMsg struct {
	resp ai.Message
	err  error
}

func (m *Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).MarginBottom(1)
	boxStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2).Width(75)

	switch m.state {
	case StateHelp:
		helpText := titleStyle.Render("TUIAI - Help & Commands") + "\n\n" +
			"TUIAI is your local markdown book assistant running in this directory.\n\n" +
			"Commands:\n" +
			"  /exit           Exit the application\n" +
			"  /read-only      Toggle read-only mode for safety\n" +
			"  /disable-delete Prevent AI from deleting files\n" +
			"  /help           Show this help screen\n\n" +
			"Press Enter or Space to return to chat."
		return boxStyle.Render(helpText)

	case StateReview:
		reviewText := titleStyle.Render("SECURITY REVIEW: Pending File Operation") + "\n\n" +
			fmt.Sprintf("Action: %s\n", strings.ToUpper(m.pendingAction)) +
			fmt.Sprintf("File:   %s\n\n", m.pendingFilename)
		if m.pendingAction == "write" {
			reviewText += fmt.Sprintf("Proposed Content Preview:\n---\n%s\n---\n\n", m.pendingContent)
		}
		reviewText += "Allow this change? [y]es / [n]o"
		return boxStyle.Render(reviewText)

	case StateChat:
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("TUIAI // Book Editor (Local)") + "\n")

		// Render recent chat log lines
		startIdx := 0
		if len(m.chatLog) > 12 {
			startIdx = len(m.chatLog) - 12
		}
		for _, line := range m.chatLog[startIdx:] {
			sb.WriteString(line + "\n")
		}

		sb.WriteString("\n")
		if m.thinking {
			sb.WriteString(m.spinner.View() + " AI is thinking...\n")
		} else {
			sb.WriteString(m.textInput.View() + "\n")
		}

		status := fmt.Sprintf("[ReadOnly: %v | NoDelete: %v]", m.sandbox.ReadOnly, m.sandbox.DisableDelete)
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(status))

		return boxStyle.Render(sb.String())
	}

	return ""
}
