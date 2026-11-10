package stamp

import (
	"fmt"
	"strings"
)

// contextLines is the number of unchanged lines shown around a change.
const contextLines = 2

// Diff returns what stamping changes in Chart.yaml, as a unified diff: the
// lines taken out start with "-", the lines put in with "+", and a few unchanged
// lines around them show where. It is empty when nothing changes.
func (p *Plan) Diff() string {
	return unifiedDiff("Chart.yaml", string(p.Before), string(p.After))
}

// lineEdit is one line of a diff: its text and what happens to it.
type lineEdit struct {
	op   byte // ' ' kept, '-' taken out, '+' put in
	text string
}

// unifiedDiff returns the lines that differ between before and after, with
// contextLines of context, in the unified format.
func unifiedDiff(name, before, after string) string {
	if before == after {
		return ""
	}
	a, b := splitLines(before), splitLines(after)
	edits := lineEdits(a, b)

	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", name, name)

	for i := 0; i < len(edits); {
		if edits[i].op == ' ' {
			i++
			continue
		}
		start := max(i-contextLines, 0)
		end := i
		for end < len(edits) {
			if edits[end].op != ' ' {
				end++
				continue
			}
			// A run of unchanged lines ends the hunk when it is long enough to
			// hold the context of both sides.
			run := end
			for run < len(edits) && edits[run].op == ' ' {
				run++
			}
			if run == len(edits) || run-end > 2*contextLines {
				break
			}
			end = run
		}
		stop := min(end+contextLines, len(edits))

		oldStart, newStart := 1, 1
		for _, e := range edits[:start] {
			if e.op != '+' {
				oldStart++
			}
			if e.op != '-' {
				newStart++
			}
		}
		oldCount, newCount := 0, 0
		for _, e := range edits[start:stop] {
			if e.op != '+' {
				oldCount++
			}
			if e.op != '-' {
				newCount++
			}
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, e := range edits[start:stop] {
			fmt.Fprintf(&out, "%c%s\n", e.op, e.text)
		}
		i = stop
	}
	return out.String()
}

// splitLines splits text into lines, without their line endings. A text that
// ends with a new line has no empty last line.
func splitLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// lineEdits returns the edits that turn a into b, from the longest common
// subsequence of their lines.
func lineEdits(a, b []string) []lineEdit {
	// common[i][j] is the length of the longest common subsequence of a[i:] and
	// b[j:].
	common := make([][]int, len(a)+1)
	for i := range common {
		common[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}

	var edits []lineEdit
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			edits = append(edits, lineEdit{op: ' ', text: a[i]})
			i++
			j++
		case common[i+1][j] >= common[i][j+1]:
			edits = append(edits, lineEdit{op: '-', text: a[i]})
			i++
		default:
			edits = append(edits, lineEdit{op: '+', text: b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		edits = append(edits, lineEdit{op: '-', text: a[i]})
	}
	for ; j < len(b); j++ {
		edits = append(edits, lineEdit{op: '+', text: b[j]})
	}
	return edits
}
