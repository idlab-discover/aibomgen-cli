package ui

import (
	"image/color"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
)

// Color palette for the application (single source of truth).
var (
	// Primary colors.
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorError     = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorHighlight = lipgloss.Color("#f048ff") // Pink

	// Text colors.
	ColorText     = lipgloss.Color("#F9FAFB") // White
	ColorTextDim  = lipgloss.Color("#9CA3AF") // Light gray
	ColorTextMute = lipgloss.Color("#6B7280") // Muted gray
)

// Text styles using lipgloss.
var (
	// Bold text.
	Bold = lipgloss.NewStyle().Bold(true)

	// Dimmed text for secondary information.
	Dim = lipgloss.NewStyle().Foreground(ColorTextDim)

	// Muted text for hints.
	Muted = lipgloss.NewStyle().Foreground(ColorTextMute)

	// Success text (green).
	Success = lipgloss.NewStyle().Foreground(ColorSuccess)

	// Warning text (amber).
	Warning = lipgloss.NewStyle().Foreground(ColorWarning)

	// Error text (red).
	Error = lipgloss.NewStyle().Foreground(ColorError)

	// Primary accent text (purple).
	Primary = lipgloss.NewStyle().Foreground(ColorPrimary)

	// Secondary accent text (cyan).
	Secondary = lipgloss.NewStyle().Foreground(ColorSecondary)

	// Highlight text.
	Highlight = lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
)

// Status indicators (functions to ensure fresh rendering).

// GetCheckMark returns a styled check mark.
func GetCheckMark() string { return Success.Render("✓") }

// GetCrossMark returns a styled cross mark.
func GetCrossMark() string { return Error.Render("✗") }

// GetWarnMark returns a styled warning mark.
func GetWarnMark() string { return Warning.Render("⚠") }

// GetInfoMark returns a styled info mark.
func GetInfoMark() string { return Secondary.Render("ℹ") }

// GetBullet returns a styled bullet point.
func GetBullet() string { return Muted.Render("•") }

// Box styles for panels and containers.
var (
	// Standard box with border.
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted).
		Padding(0, 1)

	// Highlighted box.
	HighlightBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	// Success box.
	SuccessBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Padding(0, 1)

	// Error box.
	ErrorBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Padding(0, 1)
)

// Header styles.
var (
	// Main title style.
	Title = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	// Subtitle style.
	Subtitle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Italic(true)

	// Section header.
	SectionHeader = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)
)

// Progress bar styles.
var (
	// Progress bar filled portion.
	ProgressFilled = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Background(ColorSuccess)

	// Progress bar empty portion.
	ProgressEmpty = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(lipgloss.Color("#374151"))
)

// Step status styles.
var (
	// Pending step (not started).
	StepPending = lipgloss.NewStyle().Foreground(ColorMuted)

	// Running step (in progress).
	StepRunning = lipgloss.NewStyle().Foreground(ColorSecondary)

	// Completed step.
	StepComplete = lipgloss.NewStyle().Foreground(ColorSuccess)

	// Failed step.
	StepFailed = lipgloss.NewStyle().Foreground(ColorError)

	// Skipped step.
	StepSkipped = lipgloss.NewStyle().Foreground(ColorWarning)
)

// FormatKeyValue formats a key-value pair with styling.
func FormatKeyValue(key, value string) string {
	return Dim.Render(key+": ") + value
}

// FangColorScheme returns a Fang color scheme based on the application's color palette.
func FangColorScheme(c lipgloss.LightDarkFunc) fang.ColorScheme {
	return fang.ColorScheme{
		Base:           ColorText,
		Title:          ColorPrimary,
		Description:    ColorTextDim,
		Codeblock:      c(lipgloss.Color("#1F2937"), lipgloss.Color("#2F2E36")),
		Program:        ColorSecondary,
		DimmedArgument: ColorMuted,
		Comment:        ColorMuted,
		Flag:           ColorSuccess,
		FlagDefault:    ColorTextDim,
		Command:        ColorHighlight,
		QuotedString:   ColorSecondary,
		Argument:       ColorText,
		Help:           ColorTextDim,
		Dash:           ColorMuted,
		ErrorHeader:    [2]color.Color{ColorText, ColorError},
		ErrorDetails:   ColorError,
	}
}

// BannerASCII is the ASCII art banner for the application.
const BannerASCII = `
  /$$$$$$  /$$$$$$ /$$$$$$$            /$$      /$$  /$$$$$$                                        /$$ /$$
 /$$__  $$|_  $$_/| $$__  $$          | $$$    /$$$ /$$__  $$                                      | $$|__/
| $$  \ $$  | $$  | $$  \ $$  /$$$$$$ | $$$$  /$$$$| $$  \__/  /$$$$$$  /$$$$$$$           /$$$$$$$| $$ /$$
| $$$$$$$$  | $$  | $$$$$$$  /$$__  $$| $$ $$/$$ $$| $$ /$$$$ /$$__  $$| $$__  $$ /$$$$$$ /$$_____/| $$| $$
| $$__  $$  | $$  | $$__  $$| $$  \ $$| $$  $$$| $$| $$|_  $$| $$$$$$$$| $$  \ $$|______/| $$      | $$| $$
| $$  | $$  | $$  | $$  \ $$| $$  | $$| $$\  $ | $$| $$  \ $$| $$_____/| $$  | $$        | $$      | $$| $$
| $$  | $$ /$$$$$$| $$$$$$$/|  $$$$$$/| $$ \/  | $$|  $$$$$$/|  $$$$$$$| $$  | $$        |  $$$$$$$| $$| $$
|__/  |__/|______/|_______/  \______/ |__/     |__/ \______/  \_______/|__/  |__/         \_______/|__/|__/
`
