package sanitizers

type Sanitizer interface {
	Sanitize(in map[string]interface{}) (map[string]interface{}, error)
}

func NewSanitizer(kind string) Sanitizer {
	switch kind {
	case "Pod":
		return newPodSanitizer()
	case "StatefulSet", "Deployment", "ReplicaSet", "DaemonSet", "ReplicationController", "Job":
		return newWorkloadSanitizer()
	default:
		return newDefaultSanitizer()
	}
}

type defaultSanitizer struct{}

func newDefaultSanitizer() Sanitizer {
	return defaultSanitizer{}
}

func (s defaultSanitizer) Sanitize(in map[string]interface{}) (map[string]interface{}, error) {
	ms := newMetadataSanitizer()
	in, err := ms.Sanitize(in)
	if err != nil {
		return nil, err
	}
	return in, nil
}
