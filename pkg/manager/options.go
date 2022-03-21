package manager

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type BackupManager interface {
	Dump() error
}

type BackupOptions struct {
	Ctx               string
	Config            *rest.Config
	Sanitize          bool
	DataDir           string
	Selector          string
	Target            v1beta1.TargetRef
	IncludeDependants bool
	Storage           DataWriter
}

func NewBackupManager(opt BackupOptions) BackupManager {
	switch opt.Target.Kind {
	case v1beta1.TargetKindEmpty:
		return newClusterBackupManager(opt)
	case apis.KindNamespace:
		return newNamespaceBackupManager(opt)
	default:
		return newApplicationBackupManager(opt)
	}
}

type ClusterBackupMeta struct {
	metav1.TypeMeta `json:",inline"`
	GlobalResources []ResourceGroup       `json:"globalResources,omitempty"`
	Namespaces      []NamespacedResources `json:"namespaces,omitempty"`
}

type ResourceGroup struct {
	metav1.TypeMeta `json:",inline"`
	Instances       []string `json:"instances,omitempty"`
}

type NamespacedResources struct {
	Name      string          `json:"name,omitempty"`
	Resources []ResourceGroup `json:"resources,omitempty"`
}

type NamespaceBackupMeta struct {
	metav1.TypeMeta `json:",inline"`
	Name            string          `json:"name,omitempty"`
	Resources       []ResourceGroup `json:"resources,omitempty"`
}

type DataWriter interface {
	Write(string, []byte) error
}

type FileWriter struct{}

func (w *FileWriter) Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o777)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0o644)
}
