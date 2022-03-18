package manifests

import (
	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

type ManifestDumper interface {
	Dump() error
}

func NewDumper() ManifestDumper {
	switch target.Kind {
	case v1beta1.TargetKindEmpty:
		return newClusterDumper()
	case apis.KindNamespace:
		return newNamespaceDumper()
	default:
		return newApplicationDumper()

	}
}
