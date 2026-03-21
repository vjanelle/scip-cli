package main

import (
	"fmt"
	"io"
)

type renderWritePanic struct {
	err error
}

// renderWriter collapses repetitive write-error checks in serializer-style
// code by panicking on write failure and letting the top-level render helper
// recover the original error. This is intentionally scoped to rendering code,
// where every write failure has the same outcome: abort the render.
type renderWriter struct {
	target io.Writer
}

// withRenderWriter is the single recovery boundary for renderWriter. Parser
// and indexing code still returns normal errors; only output serialization
// uses this panic/recover shortcut.
func withRenderWriter(stdout io.Writer, render func(writer renderWriter)) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			writeFailure, ok := recovered.(renderWritePanic)
			if ok {
				err = writeFailure.err
				return
			}
			panic(recovered)
		}
	}()

	render(renderWriter{target: stdout})
	return nil
}

func (writer renderWriter) Fprintf(format string, args ...any) {
	if _, err := fmt.Fprintf(writer.target, format, args...); err != nil {
		panic(renderWritePanic{err: err})
	}
}

func (writer renderWriter) Fprintln(args ...any) {
	if _, err := fmt.Fprintln(writer.target, args...); err != nil {
		panic(renderWritePanic{err: err})
	}
}
