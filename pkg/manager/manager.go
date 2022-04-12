package manager

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"

	"k8s.io/client-go/rest"
)

type BackupManager interface {
	Dump() error
}

type BackupOptions struct {
	Config            *rest.Config
	Sanitize          bool
	DataDir           string
	Selector          string
	Target            v1beta1.TargetRef
	IncludeDependants bool
	Storage           Writer
	Namespace         string
}

func NewBackupManager(opt BackupOptions) BackupManager {
	switch opt.Target.Kind {
	case v1beta1.TargetKindEmpty, apis.KindNamespace:
		return newGenericResourceBackupManager(opt)
	default:
		return newApplicationBackupManager(opt)
	}
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
