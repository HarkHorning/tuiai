package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	ws        *WorkspaceManager
	files     []string
	cursor    int
	messages  []OllamaMessage
	input     string
	status    string
	modelName string
}

var (
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	activeFile = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
)

func initialModel() (*model, error) {
	ws, err := NewWorkspaceManager()
	if err != nil {
		return nil, err
	}
	files, _ := ws.ListMarkdownFiles()

	return &model{
		ws:        ws,
		files:     files,
		modelName: "qwen2.5:32b", // Change this to your custom Modelfile name later!
		messages: []OllamaMessage{
			{Role: "system", Content: "You are tuiai, an AI book editor assistant. You have read access to all .md files in this workspace. Help the user organize, record, index, and edit their book chapters and notes."},
		},
		status: "Ready. Type your prompt and press Enter.",
	}, nil
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case " ": // Spacebar loads the highlighted .md file into context
			if len(m.files) > 0 {
				selected := m.files[m.cursor]
				content, err := m.ws.ReadMarkdown(selected)
				if err == nil {
					m.status = fmt.Sprintf("Loaded file into context: %s", selected)
					m.messages = append(m.messages, OllamaMessage{
						Role:    "user",
						Content: fmt.Sprintf("[Context File Loaded: %s]\n%s", selected, content),
					})
				}
			}
		case "enter":
			// Send user input to Ollama on Enter
			if m.input != "" {
				m.messages = append(m.messages, OllamaMessage{Role: "user", Content: m.input})
				m.input = ""
				m.status = "AI is thinking..."
				
				reply, err := CallOllama(m.modelName, m.messages)
				if err != nil {
					m.status = fmt.Sprintf("Error: %v", err)
				} else {
					m.messages = append(m.messages, OllamaMessage{Role: "assistant", Content: reply})
					m.status = "AI responded. Ready."
				}
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}
	}
	return m, nil
}

func (m *model) View() string {
	s := titleStyle.Render("=== TUIAI : Book Manuscript Manager ===") + "\n\n"
	
	fileListStr := "Workspace Markdown Files (Press SPACE to load into context):\n"
	if len(m.files) == 0 {
		fileListStr += "  (No .md files found in this directory yet)\n"
	} else {
		for i, f := range m.files {
			cursorIndicator := "  "
			if i == m.cursor {
				cursorIndicator = "> "
				fileListStr += activeFile.Render(cursorIndicator+f) + "\n"
			} else {
				fileListStr += cursorIndicator + f + "\n"
			}
		}
	}

	chatStr := "Conversation & History:\n"
	startIdx := len(m.messages) - 4
	if startIdx < 0 {
		startIdx = 0
	}
	for _, msg := range m.messages[startIdx:] {
		role := "User"
		if msg.Role == "assistant" {
			role = "AI"
		} else if msg.Role == "system" {
			continue
		}
		preview := msg.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		chatStr += fmt.Sprintf("[%s]: %s\n", role, preview)
	}

	s += boxStyle.Render(fileListStr) + "\n\n"
	s += boxStyle.Render(chatStr) + "\n\n"
	s += fmt.Sprintf("Status: %s\n", m.status)
	s += fmt.Sprintf("Prompt > %s_\n\n", m.input)
	s += "(Up/Down: Navigate files | Space: Load file | Enter: Send prompt | q: Quit)"

	return s
}
