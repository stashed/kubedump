package sanitizers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Sanitizer interface {
	Sanitize(in map[string]interface{}) (map[string]interface{}, error)
}

func NewSanitizer(r metav1.APIResource) Sanitizer {
	switch r.Kind {
	case "Pod":
		return NewPodSanitizer()
	case "StatefulSet", "Deployment", "ReplicaSet", "DaemonSet", "ReplicationController", "Job":
		return NewWorkloadSanitizer()
	default:
		return NewDefaultSanitizer()
	}
}

type defaultSanitizer struct{}

func NewDefaultSanitizer() Sanitizer {
	return defaultSanitizer{}
}

func (s defaultSanitizer) Sanitize(in map[string]interface{}) (map[string]interface{}, error) {
	ms := NewMetadataSanitizer()
	in, err := ms.Sanitize(in)
	if err != nil {
		return nil, err
	}
	return in, nil
}
