package katex

import (
	"context"
	"log"
	"strings"
	"time"

	_ "embed"

	"github.com/diamondburned/jotup/internal/extern/js"
	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

const src = "https://cdn.jsdelivr.net/npm/katex@0.15.2/dist/katex.min.js"

var renderProgram = goja.MustCompile("", `
	(function() {
		return katex.renderToString(str, {
			output: "mathml",
			displayMode: displayMode,
			throwOnError: true,
		})
	})()
`, false)

// Module is a KaTeX execution module.
type Module struct {
	rt *goja.Runtime
}

// NewModule creates a new KaTeX execution module.
func NewModule(ctx context.Context) (*Module, error) {
	katex, err := js.CompileFromURL(ctx, src)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch KaTeX")
	}

	m := Module{rt: goja.New()}

	if _, err := m.rt.RunProgram(katex); err != nil {
		return nil, errors.Wrap(err, "failed to init katex")
	}

	return &m, nil
}

// Render renders the given LaTeX string (in KaTeX variant). The returned string
// is in MathML format.
func (m *Module) Render(latex string, displayMode bool) (string, error) {
	must(m.rt.Set("str", latex))
	must(m.rt.Set("displayMode", displayMode))

	t := time.Now()
	defer func() { log.Println("KaTeX render took", time.Since(t)) }()

	// KaTeX should absolutely not throw unless something really bad happened.
	// Not too sure if panicking here is a good idea.
	v, err := m.rt.RunProgram(renderProgram)
	if err != nil {
		return "", err
	}

	ml := v.Export().(string)
	ml = strings.TrimPrefix(ml, `<span class="katex">`)
	ml = strings.TrimSuffix(ml, `</span>`)

	return ml, nil
}

func must(err error) {
	if err != nil {
		panic("BUG: " + err.Error())
	}
}
