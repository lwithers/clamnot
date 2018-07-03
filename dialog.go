package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"os/exec"
)

var (
	// successTpl, errorTpl are used to render dialog text content.
	successTpl, errorTpl *template.Template
)

// result of clam scan; passed to template.
type result struct {
	// Clam is the output from clamdscan.
	Clam string

	// Err is the error encountered when running clamdscan.
	Err string
}

const (
	// successTplT, errorTplT are the actual templates that are rendered for
	// dialog content.
	successTplT = `<tt>{{.Clam}}</tt>`
	errorTplT   = `<tt>{{.Clam}}</tt>

Error reported: <span foreground='red'>{{.Err}}</span>`
)

// dialog renders the given clamdscan output via the zenity dialog program.
func dialog(clamOutput []byte, clamErr error) {
	mode := "--info"
	tpl := successTpl
	res := result{
		Clam: string(clamOutput),
	}
	if clamErr != nil {
		mode = "--warning"
		res.Err = clamErr.Error()
		tpl = errorTpl
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteString("--text=")
	var text string
	if err := tpl.Execute(buf, res); err != nil {
		fmt.Fprintln(os.Stderr, "template err: ", err)
		text = "--text=<span foreground='red'>Error executing clamnot template</span>"
	} else {
		text = buf.String()
	}

	zenity := exec.Command("zenity", mode, "--ellipsize", text)
	if err := zenity.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "executing zenity failed: ", err)
	}
}
