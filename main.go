package main

// A simple program demonstrating the text input component from the Bubbles
// component library.

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	commentsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#0C8C6C"))
)

func readLines(r io.Reader, mutex *sync.Mutex, lines *[]string) {
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		mutex.Lock()
		*lines = append(*lines, line)
		mutex.Unlock()
	}
}

func main() {
	mutex := sync.Mutex{}
	lines := make([]string, 0, 10000000)
	go readLines(os.Stdin, &mutex, &lines)
	p := tea.NewProgram(initialModel(&mutex, &lines), tea.WithAltScreen())

	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}

type tickMsg struct{}
type errMsg error

type model struct {
	viewport  viewport.Model
	textInput textinput.Model
	mutex     *sync.Mutex
	lines     *[]string
	err       error
}

func initialModel(mutex *sync.Mutex, lines *[]string) model {
	ti := textinput.NewModel()
	ti.Placeholder = "filter for words..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 120
	// ti.BlinkSpeed = 1000000

	return model{
		textInput: ti,
		mutex:     mutex,
		lines:     lines,
		err:       nil,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter, tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func max(i, j int) int {
	if i > j {
		return i
	} else {
		return j
	}
}

func min(i, j int) int {
	if i < j {
		return i
	} else {
		return j
	}
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// TrimSpace returns a slice of the string s, with all leading
// and trailing white space removed, as defined by Unicode.
func trimTrailingSpace(s string) string {
	// Fast path for ASCII: look for the first ASCII non-space byte
	start := 0
	// Now look for the first ASCII non-space byte from the end
	stop := len(s)
	for ; stop > start; stop-- {
		c := s[stop-1]
		if c >= utf8.RuneSelf {
			return strings.TrimFunc(s[start:stop], unicode.IsSpace)
		}
		if asciiSpace[c] == 0 {
			break
		}
	}

	// At this point s[start:stop] starts and ends with an ASCII
	// non-space bytes, so we're done. Non-ASCII cases have already
	// been handled above.
	return s[start:stop]
}

func (m model) View() string {
	parts := make([]string, 0, m.viewport.Height)
	rawfilter := strings.TrimSpace(m.textInput.Value())
	terms := strings.Split(rawfilter, " ")
	filters := make([]string, 0, len(terms))
	negativeFilters := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if len(term) >= 1 && term[0] == '!' {
			if len(term) > 2 {
				negativeFilters = append(negativeFilters, strings.ToLower(term[1:]))
			}
		} else if len(term) > 0 {
			filters = append(filters, strings.ToLower(term))
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

outer:
	for i := len(*m.lines) - 1; i >= 0 && len(parts) < cap(parts)-3; i -= 1 {
		line := (*m.lines)[i]
		lineLower := strings.ToLower(line)
		for _, negativeFilter := range negativeFilters {
			if strings.Contains(lineLower, negativeFilter) {
				continue outer
			}
		}
		for _, filter := range filters {
			if !strings.Contains(lineLower, filter) {
				continue outer
			}
		}
		parts = append(parts, strings.TrimSuffix(line[0:min(m.viewport.Width-1, len(line))], "\n"))
	}
	for len(parts) < m.viewport.Height-2 {
		parts = append(parts, "")
	}
	// Reverse the filtered list
	for i := 0; i < len(parts)/2; i += 1 {
		parts[i], parts[len(parts)-1-i] = parts[len(parts)-1-i], parts[i]
	}
	parts = append(parts, m.textInput.View())
	parts = append(parts,
		commentsStyle.Render(
			strings.Join(
				[]string{
					"Including: [",
					strings.Join(filters, ", "),
					"], Excluding: [",
					strings.Join(negativeFilters, ", "), "], Total Lines: ", fmt.Sprintf("%d", len(*m.lines)), " (exclude with !term)"}, "")))

	return strings.Join(parts, "\n")
}
