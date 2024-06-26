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
	"context"
	"path/filepath"
	"time"

	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	stash "stash.appscode.dev/apimachinery/client/clientset/versioned"
	"stash.appscode.dev/apimachinery/pkg/invoker"
	"stash.appscode.dev/apimachinery/pkg/restic"
	api_util "stash.appscode.dev/apimachinery/pkg/util"
	"stash.appscode.dev/kubedump/pkg/manager"

	"github.com/spf13/cobra"
	license "go.bytebuilders.dev/license-verifier/kubernetes"
	"gomodules.xyz/flags"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	v1 "kmodules.xyz/offshoot-api/api/v1"
)

func NewCmdBackup() *cobra.Command {
	var (
		masterURL      string
		kubeconfigPath string
		opt            = options{
			setupOptions: restic.SetupOptions{
				ScratchDir:  restic.DefaultScratchDir,
				EnableCache: false,
			},
			backupOptions: restic.BackupOptions{
				Host: restic.DefaultHost,
			},
		}
	)

	cmd := &cobra.Command{
		Use:               "backup",
		Short:             "Takes a backup of Kubernetes resources",
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.EnsureRequiredFlags(cmd, "provider", "storage-secret-name", "storage-secret-namespace")
			time.Sleep(time.Second * 5)

			// prepare client
			config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				return err
			}
			opt.config = config

			opt.kubeClient, err = kubernetes.NewForConfig(config)
			if err != nil {
				return err
			}
			opt.stashClient, err = stash.NewForConfig(config)
			if err != nil {
				return err
			}

			inv, err := invoker.NewBackupInvoker(opt.stashClient, opt.invokerKind, opt.invokerName, opt.namespace)
			if err != nil {
				return err
			}

			for _, ti := range inv.GetTargetInfo() {
				if ti.Target != nil && opt.targetMatched(ti.Target.Ref, opt.targetRef) {
					var backupOutput *restic.BackupOutput
					backupOutput, err = opt.backupResources(ti.Target.Ref)
					if err != nil {
						backupOutput = &restic.BackupOutput{
							BackupTargetStatus: v1beta1.BackupTargetStatus{
								Ref: ti.Target.Ref,
								Stats: []v1beta1.HostBackupStats{
									{
										Hostname: opt.backupOptions.Host,
										Phase:    v1beta1.HostBackupFailed,
										Error:    err.Error(),
									},
								},
							},
						}
					}
					// If output directory specified, then write the output in "output.json" file in the specified directory
					if opt.outputDir != "" {
						return backupOutput.WriteOutput(filepath.Join(opt.outputDir, restic.DefaultOutputFileName))
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&masterURL, "master", masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	cmd.Flags().StringVar(&opt.namespace, "namespace", "default", "Namespace of Backup/Restore Session")
	cmd.Flags().StringVar(&opt.backupSessionName, "backupsession", opt.backupSessionName, "Name of the Backup Session")
	cmd.Flags().StringVar(&opt.storageSecret.Name, "storage-secret-name", opt.storageSecret.Name, "Name of the storage secret")
	cmd.Flags().StringVar(&opt.storageSecret.Namespace, "storage-secret-namespace", opt.storageSecret.Namespace, "Namespace of the storage secret")

	cmd.Flags().StringVar(&opt.setupOptions.Provider, "provider", opt.setupOptions.Provider, "Backend provider (i.e. gcs, s3, azure etc)")
	cmd.Flags().StringVar(&opt.setupOptions.Bucket, "bucket", opt.setupOptions.Bucket, "Name of the cloud bucket/container (keep empty for local backend)")
	cmd.Flags().StringVar(&opt.setupOptions.Endpoint, "endpoint", opt.setupOptions.Endpoint, "Endpoint for s3/s3 compatible backend or REST server URL")
	cmd.Flags().BoolVar(&opt.setupOptions.InsecureTLS, "insecure-tls", opt.setupOptions.InsecureTLS, "InsecureTLS for TLS secure s3/s3 compatible backend")
	cmd.Flags().StringVar(&opt.setupOptions.Region, "region", opt.setupOptions.Region, "Region for s3/s3 compatible backend")
	cmd.Flags().StringVar(&opt.setupOptions.Path, "path", opt.setupOptions.Path, "Directory inside the bucket where backup will be stored")
	cmd.Flags().StringVar(&opt.setupOptions.ScratchDir, "scratch-dir", opt.setupOptions.ScratchDir, "Temporary directory")
	cmd.Flags().BoolVar(&opt.setupOptions.EnableCache, "enable-cache", opt.setupOptions.EnableCache, "Specify whether to enable caching for restic")
	cmd.Flags().Int64Var(&opt.setupOptions.MaxConnections, "max-connections", opt.setupOptions.MaxConnections, "Specify maximum concurrent connections for GCS, Azure and B2 backend")

	cmd.Flags().StringVar(&opt.backupOptions.Host, "hostname", opt.backupOptions.Host, "Name of the host machine")
	cmd.Flags().StringVar(&opt.invokerKind, "invoker-kind", opt.invokerKind, "Kind of the backup invoker")
	cmd.Flags().StringVar(&opt.invokerName, "invoker-name", opt.invokerName, "Name of the respective backup invoker")
	cmd.Flags().StringVar(&opt.targetRef.Kind, "target-kind", opt.targetRef.Kind, "Kind of the Target")
	cmd.Flags().StringVar(&opt.targetRef.Name, "target-name", opt.targetRef.Name, "Name of the Target")
	cmd.Flags().StringVar(&opt.targetRef.Namespace, "target-namespace", opt.targetRef.Namespace, "Namespace of the Target")

	cmd.Flags().Int64Var(&opt.backupOptions.RetentionPolicy.KeepLast, "retention-keep-last", opt.backupOptions.RetentionPolicy.KeepLast, "Specify value for retention strategy")
	cmd.Flags().Int64Var(&opt.backupOptions.RetentionPolicy.KeepHourly, "retention-keep-hourly", opt.backupOptions.RetentionPolicy.KeepHourly, "Specify value for retention strategy")
	cmd.Flags().Int64Var(&opt.backupOptions.RetentionPolicy.KeepDaily, "retention-keep-daily", opt.backupOptions.RetentionPolicy.KeepDaily, "Specify value for retention strategy")
	cmd.Flags().Int64Var(&opt.backupOptions.RetentionPolicy.KeepWeekly, "retention-keep-weekly", opt.backupOptions.RetentionPolicy.KeepWeekly, "Specify value for retention strategy")
	cmd.Flags().Int64Var(&opt.backupOptions.RetentionPolicy.KeepMonthly, "retention-keep-monthly", opt.backupOptions.RetentionPolicy.KeepMonthly, "Specify value for retention strategy")
	cmd.Flags().Int64Var(&opt.backupOptions.RetentionPolicy.KeepYearly, "retention-keep-yearly", opt.backupOptions.RetentionPolicy.KeepYearly, "Specify value for retention strategy")
	cmd.Flags().StringSliceVar(&opt.backupOptions.RetentionPolicy.KeepTags, "retention-keep-tags", opt.backupOptions.RetentionPolicy.KeepTags, "Specify value for retention strategy")
	cmd.Flags().BoolVar(&opt.backupOptions.RetentionPolicy.Prune, "retention-prune", opt.backupOptions.RetentionPolicy.Prune, "Specify whether to prune old snapshot data")
	cmd.Flags().BoolVar(&opt.backupOptions.RetentionPolicy.DryRun, "retention-dry-run", opt.backupOptions.RetentionPolicy.DryRun, "Specify whether to test retention policy without deleting actual data")

	cmd.Flags().StringVar(&opt.outputDir, "output-dir", opt.outputDir, "Directory where output.json file will be written (keep empty if you don't need to write output in file)")

	cmd.Flags().BoolVar(&opt.sanitize, "sanitize", true, "Specify whether to remove the decorators from the resource YAML (default is true)")
	cmd.Flags().StringVar(&opt.selector, "label-selector", "", "Specify a label selector to filter the resources.")
	cmd.Flags().BoolVar(&opt.includeDependants, "include-dependants", false, "Specify whether to backup the dependants object along with their parent.")
	cmd.Flags().StringSliceVar(&opt.ignoreGroupKinds, "ignore-groupkinds", opt.ignoreGroupKinds, "Specify the groupkinds to ignore.")

	return cmd
}

func (opt *options) backupResources(targetRef v1beta1.TargetRef) (*restic.BackupOutput, error) {
	var err error
	err = license.CheckLicenseEndpoint(opt.config, licenseApiService, SupportedProducts)
	if err != nil {
		return nil, err
	}

	opt.setupOptions.StorageSecret, err = opt.kubeClient.CoreV1().Secrets(opt.storageSecret.Namespace).Get(context.TODO(), opt.storageSecret.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// if any pre-backup actions has been assigned to it, execute them
	actionOptions := api_util.ActionOptions{
		StashClient:       opt.stashClient,
		TargetRef:         targetRef,
		SetupOptions:      opt.setupOptions,
		BackupSessionName: opt.backupSessionName,
		Namespace:         opt.namespace,
	}
	err = api_util.ExecutePreBackupActions(actionOptions)
	if err != nil {
		return nil, err
	}
	// wait until the backend repository has been initialized.
	err = api_util.WaitForBackendRepository(actionOptions)
	if err != nil {
		return nil, err
	}
	// apply nice, ionice settings from env
	opt.setupOptions.Nice, err = v1.NiceSettingsFromEnv()
	if err != nil {
		return nil, err
	}
	opt.setupOptions.IONice, err = v1.IONiceSettingsFromEnv()
	if err != nil {
		return nil, err
	}

	klog.Infoln("Cleaning up directory: ", opt.dataDir)
	opt.dataDir = filepath.Join(opt.setupOptions.ScratchDir, "resources")
	if err := clearDir(opt.dataDir); err != nil {
		return nil, err
	}

	mgOpts := manager.BackupOptions{
		Config:            opt.config,
		Sanitize:          opt.sanitize,
		DataDir:           opt.dataDir,
		Target:            targetRef,
		Selector:          opt.selector,
		IncludeDependants: opt.includeDependants,
		IgnoreGroupKinds:  opt.ignoreGroupKinds,
		Storage:           manager.NewFileWriter(),
	}
	mgr := manager.NewBackupManager(mgOpts)
	if err = mgr.Dump(); err != nil {
		return nil, err
	}

	// dumped data has been stored in the interim data dir. Now, we will backup this directory using Stash.
	opt.backupOptions.BackupPaths = []string{opt.dataDir}

	// init restic wrapper
	resticWrapper, err := restic.NewResticWrapper(opt.setupOptions)
	if err != nil {
		return nil, err
	}

	return resticWrapper.RunBackup(opt.backupOptions, targetRef)
}

func (opt *options) targetMatched(tref v1beta1.TargetRef, expected v1beta1.TargetRef) bool {
	if expected.Namespace == "" && tref.Namespace != "" {
		expected.Namespace = opt.namespace
	}
	return tref.Kind == expected.Kind && tref.Name == expected.Name && tref.Namespace == expected.Namespace
}
