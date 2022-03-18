package manifests

type applicationDumper struct {
}

func newApplicationDumper() ManifestDumper {
	return applicationDumper{}
}

func (opt applicationDumper) Dump() error {
	return nil
}
