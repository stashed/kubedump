package manager

import (
	"context"
	"path/filepath"
	"strings"

	"stash.appscode.dev/elasticsearch/pkg/manager/sanitizers"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"kmodules.xyz/client-go/tools/backup"
	"sigs.k8s.io/yaml"
)

type clusterBackupManager struct {
	BackupOptions
}

func newClusterBackupManager(opt BackupOptions) BackupManager {
	return clusterBackupManager{
		BackupOptions: opt,
	}
}

func (opt clusterBackupManager) Dump() error {
	// backup cluster resources yaml into opt.backupDir
	mgr := backup.NewBackupManager(opt.Ctx, opt.Config, opt.Sanitize)

	_, err := mgr.BackupToDir(opt.DataDir)
	if err != nil {
		return err
	}
	return nil
}

type ItemList struct {
	Items []map[string]interface{} `json:"items,omitempty"`
}

type processorFunc func(relPath string, data []byte) error

func (opt clusterBackupManager) Backup(process processorFunc) error {
	// ref: https://github.com/kubernetes/ingress-nginx/blob/0dab51d9eb1e5a9ba3661f351114825ac8bfc1af/pkg/ingress/controller/launch.go#L252
	opt.Config.QPS = 1e6
	opt.Config.Burst = 1e6
	if err := rest.SetKubernetesDefaults(opt.Config); err != nil {
		return err
	}
	opt.Config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	if opt.Config.UserAgent == "" {
		opt.Config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	disClient, err := discovery.NewDiscoveryClientForConfig(opt.Config)
	if err != nil {
		return err
	}
	resourceLists, err := disClient.ServerPreferredResources()
	if err != nil {
		return err
	}
	resourceListBytes, err := yaml.Marshal(resourceLists)
	if err != nil {
		return err
	}
	err = process("resource_lists.yaml", resourceListBytes)
	if err != nil {
		return err
	}

	for _, list := range resourceLists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return err
		}
		for _, r := range list.APIResources {
			if strings.ContainsRune(r.Name, '/') {
				continue // skip subresource
			}
			if !sets.NewString(r.Verbs...).HasAll("list", "get") {
				continue
			}

			klog.V(3).Infof("Taking backup of %s apiVersion:%s kind:%s", list.GroupVersion, r.Name, r.Kind)
			opt.Config.GroupVersion = &gv
			opt.Config.APIPath = "/apis"
			if gv.Group == core.GroupName {
				opt.Config.APIPath = "/api"
			}
			client, err := rest.RESTClientFor(opt.Config)
			if err != nil {
				return err
			}
			request := client.Get().Resource(r.Name).Param("pretty", "true")
			resp, err := request.DoRaw(context.TODO())
			if err != nil {
				return err
			}
			items := &ItemList{}
			err = yaml.Unmarshal(resp, &items)
			if err != nil {
				return err
			}
			for _, item := range items.Items {
				var path string
				item["apiVersion"] = list.GroupVersion
				item["kind"] = r.Kind

				md, ok := item["metadata"]
				if ok {
					path = getPathFromSelfLink(md)
				}

				if opt.Sanitize {
					s := sanitizers.NewSanitizer(r)
					item, err := s.Sanitize(item)
					if err != nil {
						return err
					}
					delete(item, "status")
				}
				data, err := yaml.Marshal(item)
				if err != nil {
					return err
				}
				absPath := filepath.Join(opt.DataDir, path)
				err = opt.Storage.Write(absPath, data)
				if err != nil {
					return err
				}
				err = process(path, data)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func getPathFromSelfLink(md interface{}) string {
	meta, ok := md.(map[string]interface{})
	if ok {
		return meta["selfLink"].(string) + ".yaml"
	}
	return ""
}
