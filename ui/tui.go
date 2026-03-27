package ui

import (
	"fmt"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"httpinspector/models"
)

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57")).
				Bold(true).
				PaddingLeft(1).
				PaddingRight(1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			MarginBottom(1)

	detailKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

type Model struct {
	transactions []models.HTTPTransaction
	txChan       <-chan models.HTTPTransaction
	cursor       int
	width        int
	height       int
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
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.transactions)-1 {
				m.cursor++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case txMsg:
		m.transactions = append(m.transactions, models.HTTPTransaction(msg))
		return m, m.waitForTransaction()
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	leftWidth := (m.width / 3) - 2
	rightWidth := m.width - leftWidth - 4
	paneHeight := m.height - 4

	leftPaneStyle := baseStyle.Copy().Width(leftWidth).Height(paneHeight)
	rightPaneStyle := baseStyle.Copy().Width(rightWidth).Height(paneHeight)

	leftContent := titleStyle.Render("Traffic Log") + "\n"
	for i, tx := range m.transactions {
		methodStr := fmt.Sprintf("%-6s %s", tx.Method, truncate(tx.Host, leftWidth-12))
		if i == m.cursor {
			leftContent += selectedItemStyle.Render(methodStr) + "\n"
		} else {
			leftContent += itemStyle.Render(methodStr) + "\n"
		}
	}

	rightContent := titleStyle.Render("Inspector Details") + "\n"
	if len(m.transactions) > 0 && m.cursor >= 0 && m.cursor < len(m.transactions) {
		tx := m.transactions[m.cursor]

		statusColor := "46"
		if tx.StatusCode >= 400 {
			statusColor = "196"
		} else if tx.StatusCode >= 300 {
			statusColor = "214"
		} else if tx.StatusCode == 0 {
			statusColor = "245" 
		}

		statusStr := fmt.Sprintf("%d", tx.StatusCode)
		if tx.StatusCode == 0 && tx.Protocol == "HTTPS" {
			statusStr = "ENCRYPTED"
		}

		rightContent += fmt.Sprintf("%s %s\n", detailKeyStyle.Render("Protocol:     "), tx.Protocol)
		rightContent += fmt.Sprintf("%s %s\n", detailKeyStyle.Render("Method:       "), tx.Method)
		rightContent += fmt.Sprintf("%s %s\n", detailKeyStyle.Render("Host:         "), tx.Host)
		rightContent += fmt.Sprintf("%s %s\n", detailKeyStyle.Render("Path:         "), tx.Path)
		rightContent += fmt.Sprintf("%s %s\n", detailKeyStyle.Render("Status:       "), lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render(statusStr))
		
		if tx.Protocol == "HTTP" {
			rightContent += fmt.Sprintf("%s %d bytes\n", detailKeyStyle.Render("Content Len:  "), tx.ContentLength)
		}
		
		rightContent += fmt.Sprintf("%s %s\n\n", detailKeyStyle.Render("Latency:      "), tx.Duration.String())

		if tx.Protocol == "HTTPS" {
			rightContent += dimStyle.Render("Traffic is encrypted. Payload cannot be inspected.") + "\n"
		} else {
			rightContent += titleStyle.Render("Response Headers") + "\n"
			rightContent += formatHeaders(tx.ResHeaders, rightWidth-2) + "\n"

			if len(tx.ResBody) > 0 {
				rightContent += titleStyle.Render("Response Body (Preview)") + "\n"
				rightContent += dimStyle.Render(cleanBody(tx.ResBody, 200)) + "\n"
			}
		}

	} else {
		rightContent += "\nWaiting for traffic..."
	}

	left := leftPaneStyle.Render(leftContent)
	right := rightPaneStyle.Render(rightContent)

	layout := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  [↑/↓] Navigate • [q] Quit")

	return layout + "\n" + footer
}

func (m Model) waitForTransaction() tea.Cmd {
	return func() tea.Msg {
		return txMsg(<-m.txChan)
	}
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func formatHeaders(headers http.Header, max int) string {
	if len(headers) == 0 {
		return "None\n"
	}
	var s string
	count := 0
	for k, v := range headers {
		if count >= 6 {
			s += dimStyle.Render(fmt.Sprintf("... (%d more headers)\n", len(headers)-6))
			break
		}
		val := strings.Join(v, ", ")
		line := fmt.Sprintf("%s: %s", k, val)
		s += truncate(line, max) + "\n"
		count++
	}
	return s
}

func cleanBody(b []byte, maxLen int) string {
	if len(b) == 0 {
		return ""
	}
	
	for _, c := range b {
		if c == 0x00 {
			return "[Binary Data Hidden]"
		}
	}

	str := string(b)
	str = strings.ReplaceAll(str, "\r", "")
	str = strings.ReplaceAll(str, "\n", " ")
	if len(str) > maxLen {
		return str[:maxLen] + "..."
	}
	return str
}