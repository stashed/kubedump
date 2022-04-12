package sanitizers

type metadataSanitizer struct{}

func newMetadataSanitizer() Sanitizer {
	return metadataSanitizer{}
}

func (s metadataSanitizer) Sanitize(in map[string]interface{}) (map[string]interface{}, error) {
	meta, ok := in["metadata"].(map[string]interface{})
	if !ok {
		return in, nil
	}

	delete(meta, "creationTimestamp")
	delete(meta, "resourceVersion")
	delete(meta, "uid")
	delete(meta, "generateName")
	delete(meta, "generation")

	annotations, ok := meta["annotations"]
	if ok {
		meta["annotations"] = cleanUpAnnotations(annotations)
	}
	delete(meta, "managedFields")

	in["metadata"] = meta
	return in, nil
}

func cleanUpAnnotations(in interface{}) interface{} {
	m, ok := in.(map[string]string)
	if !ok {
		return in
	}
	delete(m, "controller-uid")
	delete(m, "deployment.kubernetes.io/desired-replicas")
	delete(m, "deployment.kubernetes.io/max-replicas")
	delete(m, "deployment.kubernetes.io/revision")
	delete(m, "pod-template-hash")
	delete(m, "pv.kubernetes.io/bind-completed")
	delete(m, "pv.kubernetes.io/bound-by-controller")
	return m
}
