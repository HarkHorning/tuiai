package main

import (
	"fmt"
	"os"

	"tuiai/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model, err := tui.InitialModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing TUIAI sandbox: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUIAI: %v\n", err)
		os.Exit(1)
	}
}
