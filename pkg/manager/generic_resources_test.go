package manager_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	"stash.appscode.dev/manifest-backup/pkg/manager"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_genericResource_Dump(t *testing.T) {
	tests := []struct {
		name    string
		options manager.BackupOptions
		wantErr bool
	}{
		{
			name:    "Dump full cluster",
			wantErr: false,
			options: manager.BackupOptions{
				Target: v1beta1.TargetRef{
					APIVersion: "na",
					Kind:       v1beta1.TargetKindEmpty,
					Name:       "na",
				},
				Sanitize: true,
			},
		},
		{
			name:    "Dump kube-system namespace",
			wantErr: false,
			options: manager.BackupOptions{
				Target: v1beta1.TargetRef{
					APIVersion: "v1",
					Kind:       apis.KindNamespace,
					Name:       "kube-system",
				},
				Sanitize: true,
			},
		},
	}
	var err error
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.options.Config, err = getRestConfig()
			if err != nil {
				t.Error(err)
				return
			}
			tt.options.DataDir = filepath.Join("/tmp/manifests", strings.ReplaceAll(tt.name, " ", "_"))
			tt.options.Storage = newStdOutWriter()

			mgr := manager.NewBackupManager(tt.options)
			if err := mgr.Dump(); (err != nil) != tt.wantErr {
				t.Errorf("Dump() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func newStdOutWriter() manager.Writer {
	return stdOutWriter{}
}

type stdOutWriter struct{}

func (w stdOutWriter) Write(key string, value []byte) error {
	fmt.Println("========================================")
	fmt.Println("file: ", key)
	fmt.Println(string(value))
	return nil
}

func getRestConfig() (*rest.Config, error) {
	path := os.Getenv("HOME") + "/.kube/config"
	return clientcmd.BuildConfigFromFlags("", path)
}
