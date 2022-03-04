package asciimath

import (
	"context"
	"log"
	"regexp"
	"time"

	_ "embed"

	"github.com/diamondburned/jotup/internal/extern/js"
	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

const src = "git+https://github.com/ForbesLindesay/ascii-math.git"

var renderProgram = goja.MustCompile("", `ascii_math(str).toString()`, false)

// Module is a KaTeX execution module.
type Module struct {
	rt *goja.Runtime
}

// NewModule creates a new KaTeX execution module.
func NewModule(ctx context.Context) (*Module, error) {
	m := Module{rt: goja.New()}

	if err := js.LoadFromURL(ctx, m.rt, src); err != nil {
		return nil, errors.Wrap(err, "cannot fetch KaTeX")
	}

	return &m, nil
}

var trimTagRegex = regexp.MustCompile(`^<math.*?>`)

// Render renders the given LaTeX string (in KaTeX variant). The returned string
// is in MathML format.
func (m *Module) Render(asciimath string) (string, error) {
	must(m.rt.Set("str", asciimath))

	t := time.Now()
	defer func() { log.Println("ASCIIMath render took", time.Since(t)) }()

	// KaTeX should absolutely not throw unless something really bad happened.
	// Not too sure if panicking here is a good idea.
	v, err := m.rt.RunProgram(renderProgram)
	if err != nil {
		return "", err
	}

	s := v.Export().(string)
	s = trimTagRegex.ReplaceAllLiteralString(s, "<math>")

	return s, nil
}

func must(err error) {
	if err != nil {
		panic("BUG: " + err.Error())
	}
}
