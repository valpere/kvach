// Package tui implements the interactive terminal user interface.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/valpere/kvach/internal/agent"
)

// Config configures a TUI run.
type Config struct {
	Agent *agent.Agent
	Model string
	In    io.Reader
	Out   io.Writer
}

// Run starts the interactive Bubble Tea UI.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Agent == nil {
		return errors.New("tui: agent is required")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	m := newModel(runCtx, cancel, cfg.Agent, cfg.Model)
	opts := []tea.ProgramOption{tea.WithContext(runCtx)}
	if cfg.In != nil {
		opts = append(opts, tea.WithInput(cfg.In))
	}
	if cfg.Out != nil {
		opts = append(opts, tea.WithOutput(cfg.Out))
	}

	p := tea.NewProgram(m, opts...)
	_, err := p.Run()
	return err
}

type model struct {
	ctx    context.Context
	cancel context.CancelFunc
	agent  *agent.Agent

	modelName string
	viewport  viewport.Model
	input     textarea.Model

	conversation strings.Builder
	status       string
	activeTool   string
	usage        agent.UsageInfo

	running bool
	width   int
	height  int
}

type runStartedMsg struct {
	events <-chan agent.Event
	err    error
}

type agentEventMsg struct {
	events <-chan agent.Event
	event  agent.Event
	ok     bool
}

func newModel(ctx context.Context, cancel context.CancelFunc, a *agent.Agent, modelName string) model {
	vp := viewport.New(0, 0)
	input := textarea.New()
	input.Placeholder = "Ask kvach to inspect or change your code..."
	input.Focus()
	input.Prompt = "| "
	input.SetHeight(5)
	input.CharLimit = 0
	input.ShowLineNumbers = false

	m := model{
		ctx:       ctx,
		cancel:    cancel,
		agent:     a,
		modelName: modelName,
		viewport:  vp,
		input:     input,
		status:    "idle",
	}
	m.appendConversation("Welcome to kvach. Press Ctrl+S to send, Enter for a new line, Ctrl+C to quit.\n")
	return m
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.cancel()
			return m, tea.Quit
		case tea.KeyCtrlS:
			if m.running {
				return m, nil
			}
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}

			m.input.SetValue("")
			m.status = "running"
			m.activeTool = ""
			m.running = true
			m.appendConversation(fmt.Sprintf("\nYou:\n%s\n\nAssistant:\n", prompt))
			return m, startRunCmd(m.ctx, m.agent, prompt)
		}
	}

	switch msg := msg.(type) {
	case runStartedMsg:
		if msg.err != nil {
			m.running = false
			m.status = "error"
			m.appendConversation(fmt.Sprintf("\n[error] %v\n", msg.err))
			return m, nil
		}
		return m, waitAgentEventCmd(msg.events)

	case agentEventMsg:
		if !msg.ok {
			if m.running {
				m.status = "idle"
			}
			m.running = false
			m.activeTool = ""
			return m, nil
		}
		m.handleAgentEvent(msg.event)
		return m, waitAgentEventCmd(msg.events)
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	header := lipgloss.NewStyle().Bold(true).Render("kvach")
	status := fmt.Sprintf("model: %s | status: %s", m.modelName, m.status)
	if m.activeTool != "" {
		status += " | tool: " + m.activeTool
	}
	if m.usage.InputTokens > 0 || m.usage.OutputTokens > 0 {
		status += fmt.Sprintf(" | tokens in/out: %d/%d", m.usage.InputTokens, m.usage.OutputTokens)
	}

	statusLine := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(status)
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Ctrl+S send  Enter newline  PgUp/PgDn scroll  Ctrl+C quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.viewport.View(),
		statusLine,
		help,
		m.input.View(),
	)
}

func (m *model) handleAgentEvent(evt agent.Event) {
	switch evt.Type {
	case agent.EventTextDelta:
		if s, ok := evt.Payload.(string); ok {
			m.appendConversation(s)
		}
	case agent.EventToolStarted:
		if info, ok := evt.Payload.(agent.ToolCallInfo); ok {
			m.activeTool = info.Name
			m.appendConversation(fmt.Sprintf("\n[tool] %s\n", info.Name))
		}
	case agent.EventToolCompleted:
		if info, ok := evt.Payload.(agent.ToolResultInfo); ok {
			m.activeTool = ""
			m.appendConversation(fmt.Sprintf("\n[tool done] %s (%d bytes)\n", info.Name, len(info.Content)))
		}
	case agent.EventToolError:
		if info, ok := evt.Payload.(agent.ToolErrorInfo); ok {
			m.activeTool = ""
			m.appendConversation(fmt.Sprintf("\n[tool error] %s: %s\n", info.Name, info.Message))
		}
	case agent.EventUsageUpdated:
		if info, ok := evt.Payload.(agent.UsageInfo); ok {
			m.usage = info
		}
	case agent.EventError:
		m.status = "error"
		if s, ok := evt.Payload.(string); ok {
			m.appendConversation(fmt.Sprintf("\n[error] %s\n", s))
		}
	case agent.EventDone:
		m.running = false
		m.activeTool = ""
		if reason, ok := evt.Payload.(string); ok && strings.TrimSpace(reason) != "" {
			m.status = reason
		} else {
			m.status = "completed"
		}
		m.appendConversation("\n")
	}
}

func (m *model) appendConversation(s string) {
	m.conversation.WriteString(s)
	m.viewport.SetContent(m.conversation.String())
	m.viewport.GotoBottom()
}

func (m *model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	const fixedRows = 4
	inputHeight := m.input.Height()
	vpHeight := m.height - inputHeight - fixedRows
	if vpHeight < 3 {
		vpHeight = 3
	}

	contentWidth := m.width - 2
	if contentWidth < 20 {
		contentWidth = 20
	}

	m.input.SetWidth(contentWidth)
	m.viewport.Width = contentWidth
	m.viewport.Height = vpHeight
	m.viewport.SetContent(m.conversation.String())
	m.viewport.GotoBottom()
}

func startRunCmd(ctx context.Context, a *agent.Agent, prompt string) tea.Cmd {
	return func() tea.Msg {
		events, err := a.Run(ctx, agent.RunOptions{Prompt: prompt})
		return runStartedMsg{events: events, err: err}
	}
}

func waitAgentEventCmd(events <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-events
		return agentEventMsg{events: events, event: evt, ok: ok}
	}
}
