package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type statusReporter struct {
	out   io.Writer
	tty   bool
	color bool
	phase string
}

func newStatusReporter(out io.Writer) *statusReporter {
	reporter := &statusReporter{out: out}
	if file, ok := out.(*os.File); ok {
		if stat, err := file.Stat(); err == nil && stat.Mode()&os.ModeCharDevice != 0 {
			reporter.tty = true
			reporter.color = true
		}
	}
	return reporter
}

func (r *statusReporter) Start(phase string, total int) {
	r.phase = phase
	if total < 0 {
		total = 0
	}
	if r.tty {
		r.render(0, total, "")
		return
	}
	fmt.Fprintf(r.out, "%s: 0/%d\n", phase, total)
}

func (r *statusReporter) Update(current, total int, detail string) {
	if total < 0 {
		total = 0
	}
	if current < 0 {
		current = 0
	}
	if current > total && total > 0 {
		current = total
	}
	if r.tty {
		r.render(current, total, detail)
		return
	}
	if detail != "" {
		fmt.Fprintf(r.out, "%s: %d/%d %s\n", r.phase, current, total, detail)
		return
	}
	fmt.Fprintf(r.out, "%s: %d/%d\n", r.phase, current, total)
}

func (r *statusReporter) Message(message string) {
	if message == "" {
		return
	}
	if r.tty {
		r.clearLine()
		fmt.Fprintln(r.out, r.decorate(message, colorCyan))
		return
	}
	fmt.Fprintln(r.out, message)
}

func (r *statusReporter) Done(message string) {
	if message == "" {
		return
	}
	if r.tty {
		r.clearLine()
		fmt.Fprintln(r.out, r.decorate(message, colorGreen))
		return
	}
	fmt.Fprintln(r.out, message)
}

func (r *statusReporter) render(current, total int, detail string) {
	r.clearLine()
	bar := buildBar(current, total, 24)
	progressText := fmt.Sprintf("%d/%d", current, total)
	if total == 0 {
		progressText = "0/0"
	}
	line := fmt.Sprintf("%s [%s] %s", r.decorate(r.phase, colorBlue), r.decorate(bar, colorCyan), r.decorate(progressText, colorWhite))
	if detail != "" {
		line += " " + detail
	}
	fmt.Fprint(r.out, "\r")
	fmt.Fprint(r.out, line)
}

func (r *statusReporter) clearLine() {
	if !r.tty {
		return
	}
	fmt.Fprint(r.out, "\r\x1b[2K")
}

func (r *statusReporter) decorate(text, color string) string {
	if !r.color || text == "" {
		return text
	}
	return color + text + colorReset
}

func buildBar(current, total, width int) string {
	if width <= 0 {
		width = 24
	}
	if total <= 0 {
		return strings.Repeat(".", width)
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	filled := current * width / total
	if filled > width {
		filled = width
	}
	return strings.Repeat("=", filled) + strings.Repeat(".", width-filled)
}

const (
	colorReset = "\x1b[0m"
	colorBlue  = "\x1b[34m"
	colorCyan  = "\x1b[36m"
	colorGreen = "\x1b[32m"
	colorWhite = "\x1b[37m"
)
