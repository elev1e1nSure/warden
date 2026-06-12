package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// waveSteps is the number of brightness tiers in the flowing wave gradient.
const waveSteps = 28

// Precomputed gradient cells: each entry is a single "·"/"•" already rendered
// at its brightness, so a frame is just a string concat (no per-char styling).
var (
	waveCellsGreen = buildWaveCells(0x8A, 0xB8, 0x9A)
	waveCellsBlue  = buildWaveCells(0x38, 0xBD, 0xF8)
)

func buildWaveCells(pr, pg, pb int) []string {
	// dim baseline the trough fades to
	const br, bg, bb = 0x26, 0x2A, 0x28
	cells := make([]string, waveSteps)
	for i := 0; i < waveSteps; i++ {
		t := float64(i) / float64(waveSteps-1)
		// ease-in so most of the bar stays dim and the crest pops
		e := t * t
		r := int(float64(br) + (float64(pr-br))*e)
		g := int(float64(bg) + (float64(pg-bg))*e)
		b := int(float64(bb) + (float64(pb-bb))*e)
		glyph := "·"
		if t > 0.9 {
			glyph = "•" // sparkle at the crest
		}
		col := lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b))
		cells[i] = lipgloss.NewStyle().Foreground(col).Render(glyph)
	}
	return cells
}

// renderWaveSpinner renders a 7-char bouncing wave for the status bar.
func (m model) renderWaveSpinner() string {
	const n = 7
	const lo = -2
	const hi = n + 1
	const span = hi - lo
	const cycle = span * 2
	if !m.loading {
		return FaintStyle().Render(strings.Repeat("·", n))
	}
	s := m.spinner % cycle
	var pos int
	if s < span {
		pos = lo + s
	} else {
		pos = hi - (s - span)
	}
	peak := Green
	mid := GreenMid
	faint := GreenFaint
	if m.autoMode {
		peak = Blue
		mid = BlueMid
		faint = BlueFaint
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		dist := i - pos
		if dist < 0 {
			dist = -dist
		}
		switch {
		case dist == 0:
			b.WriteString(lipgloss.NewStyle().Foreground(peak).Render("█"))
		case dist == 1:
			b.WriteString(lipgloss.NewStyle().Foreground(mid).Render("▓"))
		case dist == 2:
			b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("▒"))
		default:
			b.WriteString(FaintStyle().Render("░"))
		}
	}
	return b.String()
}

// renderFullWave renders a full-width flowing shimmer under the input bar.
// Three sine waves of different speeds/frequencies travel across the bar and
// sum into a moving brightness field — multiple soft crests drifting, not a
// single bouncing dot. Idle = static faint dots.
func (m model) renderFullWave() string {
	n := m.width
	if n < 1 {
		n = 1
	}
	if !m.loading {
		return FaintStyle().Render(strings.Repeat("·", n))
	}
	cells := waveCellsGreen
	if m.autoMode {
		cells = waveCellsBlue
	}
	maxIdx := float64(len(cells) - 1)
	phase := float64(m.spinner) * 0.20
	var b strings.Builder
	for i := 0; i < n; i++ {
		x := float64(i)
		// three travelling waves; amplitudes sum to 1 so v ∈ [-1, 1]
		v := 0.50*math.Sin(x*0.16-phase) +
			0.30*math.Sin(x*0.07+phase*0.55) +
			0.20*math.Sin(x*0.33-phase*1.6)
		t := (v + 1) / 2
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
		b.WriteString(cells[int(t*maxIdx+0.5)])
	}
	return b.String()
}

// renderStatusBar renders the bottom status bar: mode · model · hint [tokens].
func (m model) renderStatusBar() string {
	mode := AccentStyle().Render("Ask")
	if m.autoMode {
		mode = lipgloss.NewStyle().Foreground(Blue).Bold(true).Render("Auto")
	}
	dot := FaintStyle().Render(" · ")
	modelPart := lipgloss.NewStyle().Foreground(White).Render(m.modelName)

	var hint string
	switch {
	case m.escPending:
		hint = ErrorStyle().Render("Esc") + DimStyle().Render(" cancel · ctrl+c quit")
	case m.quitPending:
		hint = ErrorStyle().Render("ctrl+c") + DimStyle().Render(" quit · any key abort")
	case m.selectMode:
		keyColor := Blue
		if !m.autoMode {
			keyColor = Green
		}
		hint = DimStyle().Render("select mode · ") + lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Esc") + DimStyle().Render(" exit")
	case m.confirming:
		hint = DimStyle().Render("Y run  N cancel")
	case m.streaming:
		keyColor := Blue
		if !m.autoMode {
			keyColor = Green
		}
		hint = lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Esc") + DimStyle().Render(" interrupt")
	default:
		keyColor := Blue
		if !m.autoMode {
			keyColor = Green
		}
		key := lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Shift Tab")
		if m.autoMode {
			hint = key + DimStyle().Render("  to Ask mode")
		} else {
			hint = key + DimStyle().Render("  to Auto mode")
		}
	}

	left := mode + dot + modelPart + dot + hint

	if m.tokenLimit > 0 && m.tokenCount > 0 {
		pct := m.tokenCount * 100 / m.tokenLimit
		k := float64(m.tokenCount) / 1000.0
		tokenStr := DimStyle().Render(fmt.Sprintf("%.1fK (%d%%)", k, pct))
		leftWidth := lipgloss.Width(left)
		tokenWidth := lipgloss.Width(tokenStr)
		padding := m.width - leftWidth - tokenWidth
		if padding > 1 {
			return left + strings.Repeat(" ", padding) + tokenStr
		}
	}
	return left
}

// renderInput renders the bordered text input.
func (m model) renderInput() string {
	borderColor := Green
	if m.autoMode {
		borderColor = Blue
	}
	if m.streaming || m.confirming {
		borderColor = Faint
	}
	innerWidth := m.width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		PaddingLeft(1).
		Width(innerWidth)
	return style.Render(m.textinput.View())
}
