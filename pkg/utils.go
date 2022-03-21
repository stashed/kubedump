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

package pkg

import (
	"fmt"
	"os"
	"strings"

	stash "stash.appscode.dev/apimachinery/client/clientset/versioned"
	"stash.appscode.dev/apimachinery/pkg/restic"

	shell "gomodules.xyz/go-sh"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kmapi "kmodules.xyz/client-go/api/v1"
	appcatalog_cs "kmodules.xyz/custom-resources/client/clientset/versioned"
)

const (
	ESUser              = "ADMIN_USERNAME"
	ESPassword          = "ADMIN_PASSWORD"
	MultiElasticDumpCMD = "multielasticdump"
	ESCACertFile        = "root.pem"
	ESAuthFile          = "auth.txt"
)

type options struct {
	kubeClient    kubernetes.Interface
	stashClient   stash.Interface
	catalogClient appcatalog_cs.Interface

	namespace         string
	backupSessionName string
	interimDataDir    string
	outputDir         string
	storageSecret     kmapi.ObjectReference
	waitTimeout       int32

	sanitize bool
	config   *rest.Config
	context  string

	invokerKind string
	invokerName string
	targetKind  string
	targetName  string

	setupOptions  restic.SetupOptions
	backupOptions restic.BackupOptions
}
type sessionWrapper struct {
	sh  *shell.Session
	cmd *restic.Command
}

func (opt *options) newSessionWrapper(cmd string) *sessionWrapper {
	return &sessionWrapper{
		sh: shell.NewSession(),
		cmd: &restic.Command{
			Name: cmd,
		},
	}
}

func (session *sessionWrapper) setUserArgs(args string) {
	for _, arg := range strings.Fields(args) {
		session.cmd.Args = append(session.cmd.Args, arg)
	}
}

func clearDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("unable to clean datadir: %v. Reason: %v", dir, err)
	}
	return os.MkdirAll(dir, os.ModePerm)
}
