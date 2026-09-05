package webui

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// posterDir is where an image-baked "poster" lives (the repo's assets/poster/
// COPY'd in). Absent in the stock image, present only in a fork's or presenter's
// own build — see assets/poster/README.md.
const posterDir = "/usr/share/mdl-demo/poster"

// poster is an optional extra Tools card + sub-page, defined entirely by baked
// text files — no code change, no rebrand: it slots in as the first tile and the
// mdl-demo identity stays put. A conference shout-out, a welcome page, an "Our
// university" link. The content is trusted (the image builder wrote it, like
// recipes() and recommends()), so the HTML fragments are injected verbatim; the
// console's CSP still forbids inline styles, so they must lean on the existing
// classes, not style="" (see auth.go).
type poster struct {
	Title string        // plain text — tile title, breadcrumb, page heading
	Desc  template.HTML // one-line tile blurb (.tool-d)
	Body  template.HTML // the sub-page content
	Logo  template.HTML // inline SVG icon (or a default when none is baked)
}

// loadPoster reads the baked poster once, at boot. It returns nil — the stock
// case — when none is present (no title.html, or it is empty).
func loadPoster() *poster {
	title := strings.TrimSpace(readPosterFile("title.html"))
	if title == "" {
		return nil
	}
	p := &poster{
		Title: title,
		Desc:  template.HTML(strings.TrimSpace(readPosterFile("blurb.html"))),
		Body:  template.HTML(readPosterFile("page.html")),
		Logo:  defaultPosterIcon,
	}
	if svg := strings.TrimSpace(readPosterFile("logo.svg")); svg != "" {
		p.Logo = template.HTML(svg)
	}
	return p
}

func readPosterFile(name string) string {
	b, err := os.ReadFile(filepath.Join(posterDir, name))
	if err != nil {
		return ""
	}
	return string(b)
}

// defaultPosterIcon (a star) stands in when a poster ships no logo.svg, so the
// tile still reads as a tile and matches the line-icon set.
const defaultPosterIcon template.HTML = `<svg class="tool-i" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3l2.7 5.5 6 .9-4.3 4.2 1 6-5.4-2.8-5.4 2.8 1-6L4.3 9.4l6-.9z"/></svg>`
