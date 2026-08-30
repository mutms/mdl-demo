package webui

// Templates live one .html file per named template under templates/, embedded
// into the binary. Every value is escaped by html/template; nothing here
// interpolates raw.
//
// User-facing strings render through {{t .Lang "…"}} (tr in lang.go): the
// English text is the catalog key and its own fallback.

import (
	"embed"
	"html/template"
)

//go:embed templates
var templateFS embed.FS

// page holds the whole template set: the shell plus one named template per
// section/page. Sections are addressable on their own — that is what htmx
// fetches to refresh one card without touching the rest.
var page = template.Must(template.New("page").
	Funcs(template.FuncMap{"t": tr}).
	ParseFS(templateFS, "templates/*.html"))
