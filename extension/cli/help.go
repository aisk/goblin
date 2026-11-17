package cli

import (
	"strings"

	"github.com/aisk/goblin/object"
)

// argForm is how an Arg appears in a usage line: <name>, [<name>],
// <name>... or [<name>...].
func argForm(p *param) string {
	form := "<" + p.long() + ">"
	if p.many {
		form += "..."
	}
	if p.hasDefault {
		form = "[" + form + "]"
	}
	return form
}

// usageLine is the one-line synopsis of the program, or of one command.
func usageLine(program string, root, cmd *command) string {
	var b strings.Builder
	b.WriteString("usage: " + program)
	args := root.args
	if cmd != nil {
		b.WriteString(" " + cmd.long())
		args = cmd.args
	}
	b.WriteString(" [options]")
	if cmd == nil && len(root.commands) > 0 {
		b.WriteString(" <command> ...")
	}
	for _, p := range args {
		b.WriteString(" " + argForm(p))
	}
	return b.String()
}

// shownDefault is the "(default: X)" of a parameter, or "" when its default
// goes without saying: nil, false, 0 for a counting flag, [].
func shownDefault(p *param) string {
	if p.kind == kindFlag || !p.hasDefault || p.def == object.Nil {
		return ""
	}
	text := plain(p.def)
	if list, ok := p.def.(*object.List); ok {
		if len(list.Elements) == 0 {
			return ""
		}
		parts := make([]string, len(list.Elements))
		for i, e := range list.Elements {
			parts[i] = plain(e)
		}
		text = strings.Join(parts, ", ")
	}
	return "(default: " + text + ")"
}

func choiceNames(p *param, sep string) string {
	names := make([]string, len(p.choices))
	for i, c := range p.choices {
		names[i] = plain(c)
	}
	return strings.Join(names, sep)
}

// describe joins the right-hand text of a help row.
func describe(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

func optionLeft(p *param) string {
	left := "    "
	if p.short != 0 {
		left = "-" + string(p.short) + ", "
	}
	left += "--" + p.long()
	if p.kind == kindOption {
		placeholder := "<" + p.long() + ">"
		if len(p.choices) > 0 {
			placeholder = "<" + choiceNames(p, "|") + ">"
		}
		left += " " + placeholder
		if p.many {
			left += "..."
		}
	}
	return left
}

type helpRow struct{ left, text string }

// helpText is the --help page of the program, or of one command. The
// layout is fixed: no terminal width, no wrapping, no colour.
func helpText(program string, root, cmd *command) string {
	target := root
	if cmd != nil {
		target = cmd
	}
	type section struct {
		title string
		rows  []helpRow
	}
	var sections []section

	var arguments []helpRow
	for _, p := range target.args {
		if p.help == nil && len(p.choices) == 0 && !p.hasDefault {
			continue
		}
		choices := ""
		if len(p.choices) > 0 {
			choices = "(one of " + choiceNames(p, ", ") + ")"
		}
		arguments = append(arguments, helpRow{argForm(p), describe(deref(p.help), choices, shownDefault(p))})
	}
	if len(arguments) > 0 {
		sections = append(sections, section{"arguments:", arguments})
	}

	if cmd == nil && len(root.commands) > 0 {
		var rows []helpRow
		for _, c := range root.commands {
			rows = append(rows, helpRow{c.long(), deref(c.help)})
		}
		sections = append(sections, section{"commands:", rows})
	}

	options := root.options
	if cmd != nil {
		options = append(append([]*param(nil), cmd.options...), root.options...)
	}
	var rows []helpRow
	helpClaimed := false
	for _, p := range options {
		if p.short == 'h' {
			helpClaimed = true
		}
		required := ""
		if p.required {
			required = "(required)"
		}
		rows = append(rows, helpRow{optionLeft(p), describe(deref(p.help), shownDefault(p), required)})
	}
	helpLeft := "-h, --help"
	if helpClaimed {
		helpLeft = "    --help"
	}
	rows = append(rows, helpRow{helpLeft, "show this help and exit"})
	sections = append(sections, section{"options:", rows})

	width := 0
	for _, s := range sections {
		for _, r := range s.rows {
			width = max(width, len(r.left))
		}
	}
	lines := []string{usageLine(program, root, cmd)}
	if target.help != nil {
		lines = append(lines, "", *target.help)
	}
	for _, s := range sections {
		lines = append(lines, "", s.title)
		for _, r := range s.rows {
			line := "  " + r.left + strings.Repeat(" ", width-len(r.left)) + "  " + r.text
			lines = append(lines, strings.TrimRight(line, " "))
		}
	}
	return strings.Join(lines, "\n")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
