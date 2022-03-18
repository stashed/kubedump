package manifests

type namespaceDumper struct {
}

func newNamespaceDumper() ManifestDumper {
	return namespaceDumper{}
}

func (opt namespaceDumper) Dump() error {
	return nil
}
