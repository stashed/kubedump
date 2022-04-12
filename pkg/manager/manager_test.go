package manager_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	"stash.appscode.dev/manifest-backup/pkg/manager"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_Dump(t *testing.T) {
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
		{
			name:    "Dump by label selector",
			wantErr: false,
			options: manager.BackupOptions{
				Target: v1beta1.TargetRef{
					APIVersion: "v1",
					Kind:       apis.KindNamespace,
					Name:       "kube-system",
				},
				Sanitize: true,
				Selector: getTestLabelSelector(),
			},
		},
		{
			name:    "Dump Deployment",
			wantErr: false,
			options: manager.BackupOptions{
				Target: v1beta1.TargetRef{
					APIVersion: "apps/v1",
					Kind:       apis.KindDeployment,
					Name:       "coredns",
				},
				Sanitize:  true,
				Namespace: "kube-system",
			},
		},
		{
			name:    "Dump Deployment with dependants",
			wantErr: false,
			options: manager.BackupOptions{
				Target: v1beta1.TargetRef{
					APIVersion: "apps/v1",
					Kind:       apis.KindDeployment,
					Name:       "coredns",
				},
				Sanitize:          true,
				Namespace:         "kube-system",
				IncludeDependants: true,
			},
		},
		{
			name:    "Dump Configmap",
			wantErr: false,
			options: manager.BackupOptions{
				Target: v1beta1.TargetRef{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "coredns",
				},
				Sanitize:  true,
				Namespace: "kube-system",
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
			tt.options.Storage = manager.NewFileWriter()

			mgr := manager.NewBackupManager(tt.options)
			if err := mgr.Dump(); (err != nil) != tt.wantErr {
				t.Errorf("Dump() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func getRestConfig() (*rest.Config, error) {
	path := os.Getenv("HOME") + "/.kube/config"
	return clientcmd.BuildConfigFromFlags("", path)
}

func getTestLabelSelector() string {
	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "k8s-app",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"kube-dns"},
			},
		},
	}
	return metav1.FormatLabelSelector(&selector)
}
