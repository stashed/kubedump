package manager

import (
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
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
	KubeClient        kubernetes.Interface
	Sanitize          bool
	DataDir           string
	Selector          string
	Target            v1beta1.TargetRef
	IncludeDependants bool
	Storage           Writer
	Namespace         string
	AllNamespaces     bool
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

type Writer interface {
	Write(string, []byte) error
}

type fileWriter struct{}

func NewFileWriter() Writer {
	return fileWriter{}
}

func (w fileWriter) Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o777)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0o644)
}
