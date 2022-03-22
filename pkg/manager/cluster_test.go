package manager_test

import (
	"fmt"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	"stash.appscode.dev/manifest-backup/pkg/manager"
	"testing"
)

func Test_clusterBackupManager_Dump(t *testing.T) {

	tests := []struct {
		name    string
		options manager.BackupOptions
		wantErr bool
	}{
		{
			name:    "Empty Cluster",
			wantErr: false,
			options: manager.BackupOptions{
				Ctx:      "",
				Sanitize: false,
				DataDir:  "/tmp/manifests",
				Selector: "",
				Target: v1beta1.TargetRef{
					APIVersion: "na",
					Kind:       v1beta1.TargetKindEmpty,
					Name:       "na",
				},
				IncludeDependants: false,
				Storage:           manager.NewFileWriter(),
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
			opt := manager.NewBackupManager(tt.options)
			if err := opt.Dump(); (err != nil) != tt.wantErr {
				t.Errorf("Dump() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func getRestConfig() (*rest.Config, error) {
	path := os.Getenv("HOME") + "/.kube/config"
	return clientcmd.BuildConfigFromFlags("", path)
}

func newMemoryWriter() manager.Writer {
	return memoryWriter{}
}

type memoryWriter struct {
}

func (w memoryWriter) Write(key string, value []byte) error {
	fmt.Println("========================================")
	fmt.Println("key: ", key)
	fmt.Println("data: ", string(value))
	return nil
}
