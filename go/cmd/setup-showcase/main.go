package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	green  = lipgloss.Color("#00D47A")
	blue   = lipgloss.Color("#38BDF8")
	red    = lipgloss.Color("#ff4444")
	dim    = lipgloss.Color("#666666")
	faint  = lipgloss.Color("#444444")
	white  = lipgloss.Color("#ffffff")
	greenD = lipgloss.Color("#52B788")
)

var (
	cDim    = lipgloss.NewStyle().Foreground(dim)
	cFaint  = lipgloss.NewStyle().Foreground(faint)
	cGreen  = lipgloss.NewStyle().Foreground(green)
	cBlue   = lipgloss.NewStyle().Foreground(blue).Bold(true)
	cWhite  = lipgloss.NewStyle().Foreground(white).Bold(true)
	cRed    = lipgloss.NewStyle().Foreground(red)
	cHeader = lipgloss.NewStyle().Foreground(white).Bold(true).Padding(0, 1)
)

const sep = "────────────────────────────────────────────────────────────"

func sectionTitle(n int, name, idea string) string {
	bar := lipgloss.NewStyle().Foreground(green).Bold(true)
	num := bar.Render(fmt.Sprintf(" %d ", n))
	ttl := cWhite.Render(name)
	sub := cDim.Render("  " + idea)
	return "\n" + num + ttl + sub + "\n" + cFaint.Render(sep) + "\n"
}

type variant struct {
	title string
	body  string
}

func main() {
	variants := []variant{
		{title: "alignment + footer hints", body: v1()},
		{title: "grouped by purpose", body: v2()},
		{title: "rounded card", body: v3()},
		{title: "wizard step with help", body: v4()},
		{title: "two-column table", body: v5()},
	}

	fmt.Println()
	fmt.Println(cGreen.Bold(true).Render("  warden setup  ") + cDim.Render("— 5 design variants"))
	fmt.Println(cFaint.Render(sep))
	fmt.Println()

	for i, v := range variants {
		fmt.Print(sectionTitle(i+1, v.title, ""))
		fmt.Println(v.body)
	}
}

// ---------- v1: minimal columns + footer hints ----------
func v1() string {
	pad := func(s string, w int) string {
		n := lipgloss.Width(s)
		if n >= w {
			return s
		}
		return s + strings.Repeat(" ", w-n)
	}
	activeLabel := cGreen.Render(pad("> model", 10))
	idleLabel := cDim.Render(pad("  provider", 10))
	prov := cWhite.Render("ollama") + "  " + cDim.Render("openrouter")
	modelVal := cWhite.Render("<model>")

	body := strings.Join([]string{
		"  " + cDim.Render("warden setup"),
		"",
		idleLabel + prov,
		activeLabel + modelVal,
		"",
		cFaint.Render("  ← →  switch field    tab  next    enter  save    esc  cancel"),
	}, "\n")
	return body
}

// ---------- v2: grouped ----------
func v2() string {
	pad := func(s string, w int) string {
		n := lipgloss.Width(s)
		if n >= w {
			return s
		}
		return s + strings.Repeat(" ", w-n)
	}
	sec := cFaint.Render("  ── ")
	secL := cDim.Render(" ───────────────────────────────")
	g1 := sec + cDim.Render("connection") + secL
	g2 := sec + cDim.Render("model") + secL

	activeLab := cGreen.Render(pad("> model", 12))
	idleLab := cDim.Render(pad("  provider", 12))
	prov := cWhite.Render("ollama") + "  " + cDim.Render("openrouter")

	body := strings.Join([]string{
		"  " + cDim.Render("warden setup"),
		"",
		g1,
		"  " + idleLab + prov,
		"",
		g2,
		"  " + activeLab + cWhite.Render("<model>"),
		"",
		cFaint.Render(sep),
		cDim.Render("  enter  save     esc  cancel"),
	}, "\n")
	return body
}

// ---------- v3: rounded card ----------
func v3() string {
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(faint).
		Padding(1, 3).
		Width(60)

	title := cGreen.Render("setup")

	prov := cWhite.Render("ollama") + "  " + cDim.Render("openrouter")
	modelLine := cGreen.Render("> ") + cDim.Render("model     ") + cWhite.Render("<model>")
	provLine := "  " + cDim.Render("provider  ") + prov

	content := lipgloss.JoinVertical(lipgloss.Left,
		cDim.Render("warden · ")+title,
		"",
		provLine,
		modelLine,
	)

	cardView := card.Render(content)

	hint := cFaint.Render("  ← →  switch     enter  save     esc  cancel")
	return cardView + "\n\n" + hint
}

// ---------- v4: wizard step ----------
func v4() string {
	stepActive := cGreen.Bold(true).Render("●") + "  " + cWhite.Render("provider")
	stepIdle := cFaint.Render("○") + "  " + cDim.Render("model")
	stepLine := cDim.Render("step 1/2")

	body := strings.Join([]string{
		"  " + cDim.Render("warden setup") + "                          " + stepLine,
		"",
		"  " + stepActive,
		"  " + stepIdle,
		"",
		cFaint.Render("  ─────────────────────────────────────────────"),
		"  " + cDim.Render("Where your model runs. Local server or cloud API."),
		"",
		cFaint.Render("  ─────────────────────────────────────────────"),
		"  " + cFaint.Render("← →  switch     enter  next     esc  cancel"),
	}, "\n")
	return body
}

// ---------- v5: two-column table ----------
func v5() string {
	colL := 14
	pad := func(s string) string {
		w := lipgloss.Width(s)
		if w >= colL {
			return s
		}
		return s + strings.Repeat(" ", colL-w)
	}
	divider := cFaint.Render("  │ ")

	row := func(active bool, label, val string) string {
		lab := cDim.Render(pad(label))
		v := cWhite.Render(val)
		if active {
			lab = cGreen.Render(pad("> " + label))
		}
		return "  " + lab + divider + v
	}

	prov := cWhite.Render("ollama") + cDim.Render("  ·  ") + cDim.Render("openrouter")
	apiKey := cDim.Render("••••••••••••")

	body := strings.Join([]string{
		"  " + cDim.Render("warden setup"),
		"",
		"  " + cFaint.Render(pad("field")+"│ value"),
		"  " + cFaint.Render(strings.Repeat("─", colL)+"─┼"+strings.Repeat("─", 30)),
		row(false, "provider", prov),
		row(true, "model", "<model>"),
		row(false, "api url", "https://openrouter.ai/api/v1"),
		row(false, "api key", apiKey),
		"",
		"  " + cFaint.Render("← →  switch field    enter  save    esc  cancel"),
	}, "\n")
	return body
}
