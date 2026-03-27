package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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

	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	
	statsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

type Model struct {
	transactions []models.HTTPTransaction
	txChan       <-chan models.HTTPTransaction
	cursor       int
	width        int
	height       int
	autoScroll   bool
	infoMsg      string
	searchInput  textinput.Model
	isSearching  bool
	httpCount    int
	httpsCount   int
}

type txMsg models.HTTPTransaction

type dumpMsg struct {
	msg string
}

type clearMsg struct{}

func New(txChan <-chan models.HTTPTransaction) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter by host or method..."
	ti.CharLimit = 50
	ti.Width = 30

	return Model{
		transactions: make([]models.HTTPTransaction, 0),
		txChan:       txChan,
		autoScroll:   true,
		searchInput:  ti,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.waitForTransaction())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	filtered := m.getFilteredTransactions()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.isSearching {
			switch msg.String() {
			case "esc", "enter":
				m.isSearching = false
				m.searchInput.Blur()
				m.cursor = 0
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				cmds = append(cmds, cmd)
			}
		} else {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "/":
				m.isSearching = true
				m.searchInput.Focus()
				cmds = append(cmds, textinput.Blink)
			case "esc":
				m.searchInput.SetValue("")
				m.cursor = 0
			case "up", "k":
				m.autoScroll = false
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				m.autoScroll = false
				if m.cursor < len(filtered)-1 {
					m.cursor++
				}
			case "s":
				m.autoScroll = true
				if len(filtered) > 0 {
					m.cursor = len(filtered) - 1
				}
			case "enter":
				if len(filtered) > 0 && m.cursor >= 0 && m.cursor < len(filtered) {
					cmds = append(cmds, dumpTransaction(filtered[m.cursor]))
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case dumpMsg:
		m.infoMsg = msg.msg
		cmds = append(cmds, clearInfoMsgCmd())
	case clearMsg:
		m.infoMsg = ""
	case txMsg:
		tx := models.HTTPTransaction(msg)
		if tx.Protocol == "HTTPS" {
			m.httpsCount++
		} else {
			m.httpCount++
		}
		
		m.transactions = append(m.transactions, tx)
		if m.autoScroll {
			filteredAfterAppend := m.getFilteredTransactions()
			if len(filteredAfterAppend) > 0 {
				m.cursor = len(filteredAfterAppend) - 1
			}
		}
		cmds = append(cmds, m.waitForTransaction())
	}

	return m, tea.Batch(cmds...)
}

func (m Model) getFilteredTransactions() []models.HTTPTransaction {
	query := strings.ToLower(m.searchInput.Value())
	if query == "" {
		return m.transactions
	}

	var filtered []models.HTTPTransaction
	for _, tx := range m.transactions {
		if strings.Contains(strings.ToLower(tx.Host), query) || strings.Contains(strings.ToLower(tx.Method), query) {
			filtered = append(filtered, tx)
		}
	}
	return filtered
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	filtered := m.getFilteredTransactions()

	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	} else if len(filtered) == 0 {
		m.cursor = 0
	}

	leftWidth := (m.width / 3) - 2
	rightWidth := m.width - leftWidth - 4
	paneHeight := m.height - 5 

	leftPaneStyle := baseStyle.Copy().Width(leftWidth).Height(paneHeight)
	rightPaneStyle := baseStyle.Copy().Width(rightWidth).Height(paneHeight)

	leftHeader := titleStyle.Render("Traffic Log")
	if m.isSearching || m.searchInput.Value() != "" {
		leftHeader += dimStyle.Render(" | ") + m.searchInput.View()
	}
	leftContent := leftHeader + "\n"

	visibleItems := paneHeight - 2
	startIdx := 0
	if m.cursor >= visibleItems {
		startIdx = m.cursor - visibleItems + 1
	}

	for i := startIdx; i < len(filtered) && i < startIdx+visibleItems; i++ {
		tx := filtered[i]
		methodStr := fmt.Sprintf("%-6s %s", tx.Method, truncate(tx.Host, leftWidth-12))
		if i == m.cursor {
			leftContent += selectedItemStyle.Render(methodStr) + "\n"
		} else {
			leftContent += itemStyle.Render(methodStr) + "\n"
		}
	}

	rightContent := titleStyle.Render("Inspector Details") + "\n"
	if len(filtered) > 0 && m.cursor >= 0 && m.cursor < len(filtered) {
		tx := filtered[m.cursor]

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
				rightContent += formatBodyPreview(tx.ResBody, 15) + "\n"
			}
		}

	} else {
		if len(m.transactions) == 0 {
			rightContent += "\nWaiting for traffic..."
		} else {
			rightContent += "\nNo matching traffic found."
		}
	}

	left := leftPaneStyle.Render(leftContent)
	right := rightPaneStyle.Render(rightContent)

	layout := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	statsBar := statsStyle.Render(fmt.Sprintf(" HTTP: %d | HTTPS: %d | TOTAL: %d ", m.httpCount, m.httpsCount, len(m.transactions)))
	
	scrollState := "LIVE"
	if !m.autoScroll {
		scrollState = "PAUSED"
	}

	footerText := fmt.Sprintf("  [%s] • [↑/↓] Navigate • [/] Search • [Esc] Clear • [Enter] Dump • [q] Quit", scrollState)
	if m.isSearching {
		footerText = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Render("  SEARCHING... Press [Enter] or [Esc] to exit search mode.")
	}

	if m.infoMsg != "" {
		footerText += "  |  " + successStyle.Render(m.infoMsg)
	}
	
	footerLayout := lipgloss.JoinHorizontal(lipgloss.Left, statsBar, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(footerText))

	return layout + "\n" + footerLayout
}

func (m Model) waitForTransaction() tea.Cmd {
	return func() tea.Msg {
		return txMsg(<-m.txChan)
	}
}

func dumpTransaction(tx models.HTTPTransaction) tea.Cmd {
	return func() tea.Msg {
		err := os.MkdirAll("dumps", os.ModePerm)
		if err != nil {
			return dumpMsg{msg: "Error creating directory"}
		}

		cleanHost := strings.ReplaceAll(tx.Host, ":", "_")
		cleanHost = strings.ReplaceAll(cleanHost, "/", "_")
		filename := fmt.Sprintf("dump_%s_%d.txt", cleanHost, time.Now().Unix())
		filepath := filepath.Join("dumps", filename)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("=== INSPECTOR DUMP ===\n"))
		sb.WriteString(fmt.Sprintf("ID: %s\n", tx.ID))
		sb.WriteString(fmt.Sprintf("Time: %s\n", tx.RequestTime.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("Target: %s %s%s\n", tx.Method, tx.Host, tx.Path))
		sb.WriteString(fmt.Sprintf("Status: %d\n", tx.StatusCode))
		sb.WriteString(fmt.Sprintf("Latency: %s\n\n", tx.Duration.String()))

		sb.WriteString("=== REQUEST HEADERS ===\n")
		for k, v := range tx.ReqHeaders {
			sb.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", ")))
		}

		if len(tx.ReqBody) > 0 {
			sb.WriteString("\n=== REQUEST BODY ===\n")
			sb.Write(tx.ReqBody)
			sb.WriteString("\n")
		}

		sb.WriteString("\n=== RESPONSE HEADERS ===\n")
		for k, v := range tx.ResHeaders {
			sb.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", ")))
		}

		if len(tx.ResBody) > 0 {
			sb.WriteString("\n=== RESPONSE BODY ===\n")
			
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, tx.ResBody, "", "  "); err == nil {
				sb.Write(prettyJSON.Bytes())
			} else {
				sb.Write(tx.ResBody)
			}
			sb.WriteString("\n")
		}

		err = os.WriteFile(filepath, []byte(sb.String()), 0644)
		if err != nil {
			return dumpMsg{msg: "Failed to write dump file"}
		}

		return dumpMsg{msg: fmt.Sprintf("Exported to %s", filepath)}
	}
}

func clearInfoMsgCmd() tea.Cmd {
	return tea.Tick(time.Second*3, func(_ time.Time) tea.Msg {
		return clearMsg{}
	})
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

func formatBodyPreview(b []byte, maxLines int) string {
	if len(b) == 0 {
		return ""
	}

	for _, c := range b {
		if c == 0x00 {
			return dimStyle.Render("[Binary Data Hidden]")
		}
	}

	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err == nil {
		b = out.Bytes()
	}

	str := string(b)
	str = strings.ReplaceAll(str, "\r", "")
	lines := strings.Split(str, "\n")
	
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, "... (Press Enter to export full payload)")
	}

	return dimStyle.Render(strings.Join(lines, "\n"))
}