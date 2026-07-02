package tui

import "github.com/charmbracelet/lipgloss"

var (
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#06B6D4")
	successColor   = lipgloss.Color("#10B981")
	warningColor   = lipgloss.Color("#F59E0B")
	errorColor     = lipgloss.Color("#EF4444")
	mutedColor     = lipgloss.Color("#6B7280")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(primaryColor).MarginBottom(1)
	selectedStyle = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
	cleanStyle = lipgloss.NewStyle().Foreground(successColor)
	dirtyStyle = lipgloss.NewStyle().Foreground(warningColor)
	lockedStyle = lipgloss.NewStyle().Foreground(errorColor)
	mutedStyle = lipgloss.NewStyle().Foreground(mutedColor)
	errorStyle = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
	helpStyle = lipgloss.NewStyle().Foreground(mutedColor).MarginTop(1)

	infoStyle = lipgloss.NewStyle().Foreground(successColor)
	mainStyle = lipgloss.NewStyle().Foreground(primaryColor)
)