package sanitizers

import "fmt"

type metadataSanitizer struct{}

func NewMetadataSanitizer() Sanitizer {
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

	annotation, ok := meta["annotations"]
	if ok {
		annotations, ok := annotation.(map[string]string)
		if !ok {
			return nil, fmt.Errorf("failed to parse annotations")
		}
		cleanUpDecorators(annotations)
	}
	in["metadata"] = meta
	return in, nil
}

func cleanUpObjectMeta(md interface{}) {
	meta, ok := md.(map[string]interface{})
	if !ok {
		return
	}
	delete(meta, "creationTimestamp")
	delete(meta, "resourceVersion")
	delete(meta, "uid")
	delete(meta, "generateName")
	delete(meta, "generation")
	annotation, ok := meta["annotations"]
	if !ok {
		return
	}
	annotations, ok := annotation.(map[string]string)
	if !ok {
		return
	}
	cleanUpDecorators(annotations)
}

func cleanUpDecorators(m map[string]string) {
	delete(m, "controller-uid")
	delete(m, "deployment.kubernetes.io/desired-replicas")
	delete(m, "deployment.kubernetes.io/max-replicas")
	delete(m, "deployment.kubernetes.io/revision")
	delete(m, "pod-template-hash")
	delete(m, "pv.kubernetes.io/bind-completed")
	delete(m, "pv.kubernetes.io/bound-by-controller")
}
