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

// styleWrapper wraps a lipgloss style.
type styleWrapper struct {
	style lipgloss.Style
}

// Render renders the string with the style.
func (s styleWrapper) Render(str string) string {
	return s.style.Render(str)
}

// Bold returns a new style with bold enabled.
func (s styleWrapper) Bold(v bool) styleWrapper {
	return styleWrapper{s.style.Bold(v)}
}

// Text styles using lipgloss.
var (
	// Bold text.
	Bold = styleWrapper{lipgloss.NewStyle().Bold(true)}

	// Dimmed text for secondary information.
	Dim = styleWrapper{lipgloss.NewStyle().Foreground(ColorTextDim)}

	// Muted text for hints.
	Muted = styleWrapper{lipgloss.NewStyle().Foreground(ColorTextMute)}

	// Success text (green).
	Success = styleWrapper{lipgloss.NewStyle().Foreground(ColorSuccess)}

	// Warning text (amber).
	Warning = styleWrapper{lipgloss.NewStyle().Foreground(ColorWarning)}

	// Error text (red).
	Error = styleWrapper{lipgloss.NewStyle().Foreground(ColorError)}

	// Primary accent text (purple).
	Primary = styleWrapper{lipgloss.NewStyle().Foreground(ColorPrimary)}

	// Secondary accent text (cyan).
	Secondary = styleWrapper{lipgloss.NewStyle().Foreground(ColorSecondary)}

	// Highlight text.
	Highlight = styleWrapper{lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)}
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
type boxWrapper struct {
	style lipgloss.Style
}

func (b boxWrapper) Render(str string) string {
	return b.style.Render(str)
}

var (
	// Standard box with border.
	Box = boxWrapper{lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted).
		Padding(0, 1)}

	// Highlighted box.
	HighlightBox = boxWrapper{lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)}

	// Success box.
	SuccessBox = boxWrapper{lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Padding(0, 1)}

	// Error box.
	ErrorBox = boxWrapper{lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Padding(0, 1)}
)

// Header styles.
var (
	// Main title style.
	Title = styleWrapper{lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)}

	// Subtitle style.
	Subtitle = styleWrapper{lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Italic(true)}

	// Section header.
	SectionHeader = styleWrapper{lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)}
)

// Progress bar styles.
var (
	// Progress bar filled portion.
	ProgressFilled = styleWrapper{lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Background(ColorSuccess)}

	// Progress bar empty portion.
	ProgressEmpty = styleWrapper{lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(lipgloss.Color("#374151"))}
)

// Step status styles.
var (
	// Pending step (not started).
	StepPending = styleWrapper{lipgloss.NewStyle().Foreground(ColorMuted)}

	// Running step (in progress).
	StepRunning = styleWrapper{lipgloss.NewStyle().Foreground(ColorSecondary)}

	// Completed step.
	StepComplete = styleWrapper{lipgloss.NewStyle().Foreground(ColorSuccess)}

	// Failed step.
	StepFailed = styleWrapper{lipgloss.NewStyle().Foreground(ColorError)}

	// Skipped step.
	StepSkipped = styleWrapper{lipgloss.NewStyle().Foreground(ColorWarning)}
)

// StyledText applies a lipgloss style to a string.
func StyledText(s string, style lipgloss.Style) string {
	return style.Render(s)
}

// FormatKeyValue formats a key-value pair with styling.
func FormatKeyValue(key, value string) string {
	return Dim.Render(key+": ") + value
}

// FormatStatus formats a status message with an appropriate icon.
func FormatStatus(status, message string) string {
	var icon string
	switch status {
	case "success":
		icon = GetCheckMark()
	case "error":
		icon = GetCrossMark()
	case "warning":
		icon = GetWarnMark()
	case "info":
		icon = GetInfoMark()
	default:
		icon = GetBullet()
	}
	return icon + " " + message
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

// RenderGradientBanner renders the banner with secondary color (cyan).
func RenderGradientBanner(banner string) string {
	// did not find a good way to do gradient in lipgloss, so using secondary color for now.
	return Secondary.Render(banner)
}
