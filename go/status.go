package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// waveSteps is the number of brightness tiers in the flowing wave gradient.
const waveSteps = 28

// sideMargin is the horizontal inset (each side) of the centered input column.
func (m model) sideMargin() int {
	mg := 4
	if m.width < 40 {
		mg = 1
	}
	if mg*2 >= m.width {
		mg = 0
	}
	return mg
}

// barWidth is the total visible width of the input box (border + padding +
// content). The box is centered in the terminal, leaving sideMargin each side.
func (m model) barWidth() int {
	w := m.width - 2*m.sideMargin()
	if w < 1 {
		w = 1
	}
	return w
}

// inputContentWidth is the textarea render width inside the box:
// barWidth minus border(1) + padL(2) + padR(1).
func (m model) inputContentWidth() int {
	w := m.barWidth() - 4
	if w < 1 {
		w = 1
	}
	return w
}

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
	// a touch narrower than the input bar, centered under it
	n := m.barWidth() - 4
	if n < 1 {
		n = 1
	}
	if !m.loading {
		line := FaintStyle().Render(strings.Repeat("·", n))
		return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, line)
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
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, b.String())
}

// inputBg is the shared background of the input box and its status footer.
const inputBg = lipgloss.Color("#1e1e1e")

// renderStatusContent builds the status line (mode · model · hint [tokens]) as a
// string exactly `width` cells wide. Every segment carries bg so the background
// fills under colored text too; the gap is padded manually (no lipgloss Width,
// which clips when combined with padding).
func (m model) renderStatusContent(width int, bg lipgloss.Color) string {
	fg := func(c lipgloss.Color, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(c).Background(bg)
		if bold {
			s = s.Bold(true)
		}
		return s
	}
	dim := func(s string) string { return fg(Dim, false).Render(s) }
	bgStyle := lipgloss.NewStyle().Background(bg)

	modeColor, modeLabel := Green, "Ask"
	if m.autoMode {
		modeColor, modeLabel = Blue, "Auto"
	}
	keyColor := Green
	if m.autoMode {
		keyColor = Blue
	}

	mode := fg(modeColor, true).Render(modeLabel)
	dot := fg(Faint, false).Render(" · ")
	modelPart := fg(White, false).Render(m.modelName)

	var hint string
	switch {
	case m.escPending:
		hint = fg(Red, false).Render("Esc") + dim(" cancel · ctrl+c quit")
	case m.quitPending:
		hint = fg(Red, false).Render("ctrl+c") + dim(" quit · any key abort")
	case m.selectMode:
		hint = dim("Select mode · ") + fg(keyColor, true).Render("Esc") + dim(" exit")
	case m.confirming:
		hint = dim("Y run  N cancel")
	case m.streaming:
		hint = fg(keyColor, true).Render("Esc") + dim(" interrupt")
	default:
		key := fg(keyColor, true).Render("Shift Tab")
		if m.autoMode {
			hint = key + dim("  to Ask mode")
		} else {
			hint = key + dim("  to Auto mode")
		}
	}

	left := mode + dot + modelPart + dot + hint
	leftW := lipgloss.Width(left)

	if pad := width - leftW; pad > 0 {
		return left + bgStyle.Render(strings.Repeat(" ", pad))
	}
	return left
}

// renderTokenLine renders a right-aligned dim token usage line below the input box.
func (m model) renderTokenLine() string {
	if m.tokenLimit <= 0 || m.tokenCount <= 0 {
		return ""
	}
	pct := m.tokenCount * 100 / m.tokenLimit
	k := float64(m.tokenCount) / 1000.0
	text := fmt.Sprintf("%.1fK (%d%%)", k, pct)
	styled := DimStyle().Render(text)
	padded := lipgloss.PlaceHorizontal(m.barWidth(), lipgloss.Right, styled)
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, padded)
}

// renderInput renders the input box with an integrated status footer, centered in the terminal.
// Layout: ▌ top-pad / textarea / blank spacer / status / bottom-pad
func (m model) renderInput() string {
	accentColor := Green
	if m.autoMode {
		accentColor = Blue
	}
	if m.streaming || m.confirming {
		accentColor = Faint
	}
	cw := m.inputContentWidth()

	// Single bordered box: top-pad, textarea, blank spacer, status, bottom-pad.
	// One left border drawn across all lines — no separate footer bar. The
	// bottom padding gives a full-width strip of bg below the status line.
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.Border{Left: "▌"}).
		BorderLeft(true).
		BorderForeground(accentColor).
		Background(inputBg).
		PaddingLeft(2).
		PaddingRight(1).
		PaddingTop(1).
		PaddingBottom(1).
		Width(cw)

	// Measure the box's actual content-text width so the status line fills it
	// exactly (right edge lines up regardless of how lipgloss treats padding).
	boxWidth := lipgloss.Width(boxStyle.Render(m.textinput.View()))
	statusW := boxWidth - 4
	if statusW < 1 {
		statusW = 1
	}

	content := m.textinput.View() + "\n\n" + m.renderStatusContent(statusW, inputBg)
	box := boxStyle.Render(content)
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box)
}
