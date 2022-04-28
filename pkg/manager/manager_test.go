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

package manager_test

import (
	"fmt"
	"gomodules.xyz/go-sh"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	"stash.appscode.dev/kubedump/pkg/manager"

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
					Namespace:  "kube-system",
				},
				Sanitize: true,
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
					Namespace:  "kube-system",
				},
				Sanitize:          true,
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
					Namespace:  "kube-system",
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
			tt.options.DataDir = filepath.Join("/tmp/resources", strings.ReplaceAll(tt.name, " ", "_"))
			tt.options.Storage = manager.NewFileWriter()

			mgr := manager.NewBackupManager(tt.options)
			if err := mgr.Dump(); (err != nil) != tt.wantErr {
				t.Errorf("Dump() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func getRestConfig() (*rest.Config, error) {
	path := homedir.HomeDir() + "/.kube/config"
	fmt.Println("kubeconfigPath: ", path)
	fmt.Println("HOME: ", os.Getenv("HOME"))

	_ = sh.Command("ls", "/home/runner/.kube").Run()
	file, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fmt.Println("File: ", file.Name())
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
