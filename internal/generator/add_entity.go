package generator

import "fmt"

// addEntityCmd implements `add entity NAME prop:type... --context CTX`, the
// ar-tier context/arch engine's entity add (there is no non-context entity
// profile anymore).
func addEntityCmd(r addRequest, m *Manifest, d *data) error {
	contextName, err := contextOption(r.Args)
	if err != nil {
		return err
	}
	if contextName == "" {
		return fmt.Errorf("entity requires --context ContextName (ar-tier); run 'aspgen add entity --help'")
	}
	ctx, ok := findContext(m.Contexts, contextName)
	if !ok {
		return fmt.Errorf("bounded context %q does not exist; add it first", contextName)
	}
	if ctx.Arch == "" {
		return fmt.Errorf("context %q has no arch tier; run 'aspgen add context %s --arch ar' first", contextName, contextName)
	}
	return addContextEntityCmd(r, m, ctx, d)
}
