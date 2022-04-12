package sanitizers

import "fmt"

type workloadSanitizer struct{}

func newWorkloadSanitizer() Sanitizer {
	return workloadSanitizer{}
}

func (s workloadSanitizer) Sanitize(in map[string]interface{}) (map[string]interface{}, error) {
	ms := newMetadataSanitizer()
	in, err := ms.Sanitize(in)
	if err != nil {
		return nil, err
	}

	spec, ok := in["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to parse workload spec")
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to parse pod template")
	}
	ps := newPodSanitizer()
	template, err = ps.Sanitize(template)
	if err != nil {
		return nil, err
	}
	spec["template"] = template
	in["spec"] = spec
	return in, nil
}
