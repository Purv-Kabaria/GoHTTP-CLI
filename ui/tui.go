package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"httpinspector/models"
)

// Truncate long IPv6 addresses to maintain UI columns
func formatStr(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return fmt.Sprintf("%-*s", max, s)
}

type Model struct {
	transactions []models.HTTPTransaction
	txChan       <-chan models.HTTPTransaction
}

type txMsg models.HTTPTransaction

func New(txChan <-chan models.HTTPTransaction) Model {
	return Model{
		transactions: make([]models.HTTPTransaction, 0),
		txChan:       txChan,
	}
}

func (m Model) Init() tea.Cmd {
	return m.waitForTransaction()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case txMsg:
		m.transactions = append(m.transactions, models.HTTPTransaction(msg))
		return m, m.waitForTransaction()
	}
	return m, nil
}

func (m Model) View() string {
	s := "Live HTTP Traffic\n\n"
	s += fmt.Sprintf("%-18s %-6s %-30s %-8s %s\n", "CLIENT IP", "METHOD", "HOST", "STATUS", "LATENCY")
	s += "--------------------------------------------------------------------------------\n"

	for _, tx := range m.transactions {
		ip := formatStr(tx.SourceIP, 18)
		host := formatStr(tx.Host, 30)
		status := fmt.Sprintf("%d", tx.StatusCode)
		latency := fmt.Sprintf("%dms", tx.Duration.Milliseconds())
		
		s += fmt.Sprintf("%-18s %-6s %-30s %-8s %s\n", ip, tx.Method, host, status, latency)
	}
	s += "\nPress q to quit.\n"
	return s
}

func (m Model) waitForTransaction() tea.Cmd {
	return func() tea.Msg {
		return txMsg(<-m.txChan)
	}
}