package testutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	
	// Colors
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

// Pretty printer for demo output
type PrettyPrinter struct {
	indent string
	typewriterDelay time.Duration
	typewriterEnabled bool
	interactive bool
}

func NewPrettyPrinter() *PrettyPrinter {
	return &PrettyPrinter{
		indent: "  ",
		typewriterDelay: 15 * time.Millisecond,
		typewriterEnabled: true,
		interactive: false,
	}
}

// SetInteractive enables or disables interactive mode (typewriter + pauses)
func (p *PrettyPrinter) SetInteractive(interactive bool) {
	p.interactive = interactive
	p.typewriterEnabled = interactive
}

// EnableTypewriter enables or disables the typewriter effect
func (p *PrettyPrinter) EnableTypewriter(enabled bool) {
	p.typewriterEnabled = enabled
}

// SetTypewriterDelay sets the delay between characters
func (p *PrettyPrinter) SetTypewriterDelay(delay time.Duration) {
	p.typewriterDelay = delay
}

// typewrite prints text with a typewriter effect
func (p *PrettyPrinter) typewrite(text string) {
	if !p.typewriterEnabled {
		fmt.Print(text)
		return
	}
	
	for _, char := range text {
		fmt.Print(string(char))
		time.Sleep(p.typewriterDelay)
	}
}

// typewriteln prints text with a typewriter effect and adds a newline
func (p *PrettyPrinter) typewriteln(text string) {
	p.typewrite(text)
	fmt.Println()
}

// Section prints a major section header
func (p *PrettyPrinter) Section(title string) {
	fmt.Println()
	fmt.Println()
	p.typewriteln(p.indent + strings.Repeat("═", 76))
	p.typewriteln(fmt.Sprintf("%s%s%s%s%s", p.indent, Bold, Blue, title, Reset))
	p.typewriteln(p.indent + strings.Repeat("═", 76))
	fmt.Println()
}

// SubSection prints a subsection header
func (p *PrettyPrinter) SubSection(title string) {
	fmt.Println()
	p.typewriteln(fmt.Sprintf("%s%s%s▶ %s%s", p.indent, Bold, Cyan, title, Reset))
	p.typewriteln(p.indent + strings.Repeat("─", 60))
	fmt.Println()
}

// Success prints a success message with checkmark
func (p *PrettyPrinter) Success(message string) {
	p.typewriteln(fmt.Sprintf("%s%s✓%s %s", p.indent, Green, Reset, message))
}

// Error prints an error message with X
func (p *PrettyPrinter) Error(message string) {
	p.typewriteln(fmt.Sprintf("%s%s✗%s %s", p.indent, Red, Reset, message))
}

// Info prints an info message
func (p *PrettyPrinter) Info(message string) {
	p.typewriteln(fmt.Sprintf("%s%s", p.indent, message))
}

// Warning prints a warning message
func (p *PrettyPrinter) Warning(message string) {
	p.typewriteln(fmt.Sprintf("%s%s⚠%s  %s", p.indent, Yellow, Reset, message))
}

// Code prints a code block with syntax highlighting
func (p *PrettyPrinter) Code(language, code string) {
	fmt.Println()
	p.typewriteln(fmt.Sprintf("%s%s```%s%s", p.indent, Dim, language, Reset))
	
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		highlighted := p.highlightGoCode(line)
		p.typewriteln(fmt.Sprintf("%s%s", p.indent, highlighted))
	}
	
	p.typewriteln(fmt.Sprintf("%s%s```%s", p.indent, Dim, Reset))
	fmt.Println()
}

// highlightGoCode applies basic syntax highlighting to Go code
func (p *PrettyPrinter) highlightGoCode(line string) string {
	// Keywords
	keywords := []string{"func", "type", "struct", "interface", "return", "if", "else", "for", "range", "package", "import", "var", "const", "new", "make"}
	for _, kw := range keywords {
		line = strings.ReplaceAll(line, " "+kw+" ", fmt.Sprintf(" %s%s%s ", Blue, kw, Reset))
		if strings.HasPrefix(line, kw+" ") {
			line = fmt.Sprintf("%s%s%s", Blue, kw, Reset) + line[len(kw):]
		}
	}
	
	// Types
	types := []string{"string", "int", "float64", "bool", "error", "time.Time", "[]byte", "map"}
	for _, t := range types {
		line = strings.ReplaceAll(line, " "+t, fmt.Sprintf(" %s%s%s", Cyan, t, Reset))
		line = strings.ReplaceAll(line, "("+t, fmt.Sprintf("(%s%s%s", Cyan, t, Reset))
		line = strings.ReplaceAll(line, "["+t, fmt.Sprintf("[%s%s%s", Cyan, t, Reset))
	}
	
	// Strings (basic)
	if strings.Contains(line, `"`) {
		parts := strings.Split(line, `"`)
		result := parts[0]
		for i := 1; i < len(parts); i++ {
			if i%2 == 1 {
				result += fmt.Sprintf(`%s"%s"%s`, Green, parts[i], Reset)
			} else {
				result += `"` + parts[i]
			}
		}
		line = result
	}
	
	// Comments
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx] + fmt.Sprintf("%s%s%s", Dim, line[idx:], Reset)
	}
	
	return line
}

// Feature prints a feature demonstration
func (p *PrettyPrinter) Feature(icon, title, description string) {
	p.typewriteln(fmt.Sprintf("%s%s %s%s%s", p.indent, icon, Bold, title, Reset))
	if description != "" {
		p.typewriteln(fmt.Sprintf("%s   %s%s%s", p.indent, Dim, description, Reset))
	}
}

// Step prints a step in a process
func (p *PrettyPrinter) Step(number int, description string) {
	p.typewriteln(fmt.Sprintf("%s%s%d.%s %s", p.indent, Bold, number, Reset, description))
}

// Box draws a box around content with consistent formatting
func (p *PrettyPrinter) Box(title, content string) {
	fmt.Println()
	
	// Fixed width for consistency
	boxWidth := 72
	contentWidth := boxWidth - 4 // Account for "│ " and " │"
	
	// Create formatted title with ANSI codes
	formattedTitle := fmt.Sprintf("%s%s%s", Bold, title, Reset)
	
	// Calculate padding based on visible title length (without ANSI codes)
	// The -4 accounts for "┌─ " and " ┐"
	titlePadding := boxWidth - len(title) - 4
	if titlePadding < 2 {
		titlePadding = 2
	}
	
	// Top border with formatted title
	p.typewriteln(fmt.Sprintf("%s┌─ %s %s┐", 
		p.indent, 
		formattedTitle,
		strings.Repeat("─", titlePadding)))
	
	// Content lines
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Handle long lines by wrapping
		if len(line) > contentWidth {
			// Print the first part
			p.typewriteln(fmt.Sprintf("%s│ %-*s │", p.indent, contentWidth, line[:contentWidth]))
			// Wrap the rest
			remaining := line[contentWidth:]
			for len(remaining) > 0 {
				if len(remaining) > contentWidth {
					p.typewriteln(fmt.Sprintf("%s│ %-*s │", p.indent, contentWidth, remaining[:contentWidth]))
					remaining = remaining[contentWidth:]
				} else {
					p.typewriteln(fmt.Sprintf("%s│ %-*s │", p.indent, contentWidth, remaining))
					remaining = ""
				}
			}
		} else {
			p.typewriteln(fmt.Sprintf("%s│ %-*s │", p.indent, contentWidth, line))
		}
	}
	
	// Bottom border
	p.typewriteln(fmt.Sprintf("%s└%s┘", p.indent, strings.Repeat("─", boxWidth)))
	fmt.Println()
}

// Stats prints statistics in a nice format
func (p *PrettyPrinter) Stats(title string, stats map[string]interface{}) {
	fmt.Println()
	p.typewriteln(fmt.Sprintf("%s%s📊 %s%s", p.indent, Bold, title, Reset))
	p.typewriteln(p.indent + strings.Repeat("─", 40))
	
	// Find the longest key for alignment
	maxKeyLen := 0
	for key := range stats {
		if len(key) > maxKeyLen {
			maxKeyLen = len(key)
		}
	}
	
	// Print stats with consistent alignment
	for key, value := range stats {
		p.typewriteln(fmt.Sprintf("%s%-*s : %s%v%s", 
			p.indent, 
			maxKeyLen + 2, 
			key,
			Green, value, Reset))
	}
	fmt.Println()
}

// WaitForEnter waits for the user to press Enter (only in interactive mode)
func (p *PrettyPrinter) WaitForEnter(message string) {
	if !p.interactive {
		return
	}
	
	if message == "" {
		message = "Press Enter to continue..."
	}
	
	fmt.Println()
	p.typewrite(p.indent + message)
	
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	
	// Clear the line
	fmt.Printf("\033[1A\033[2K")
}