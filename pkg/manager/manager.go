/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Free Trial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Free-Trial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package manager

import (
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
	return os.WriteFile(path, data, 0o644)
}
