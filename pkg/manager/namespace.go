package manager

import (
	"context"
	"gomodules.xyz/sets"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
)

type namespaceBackupManager struct {
	BackupOptions
}

func newNamespaceBackupManager(opt BackupOptions) BackupManager {
	return namespaceBackupManager{
		BackupOptions: opt,
	}
}

func (opt namespaceBackupManager) Dump() error {
	opt.Config.QPS = 1e6
	opt.Config.Burst = 1e6
	if err := rest.SetKubernetesDefaults(opt.Config); err != nil {
		return err
	}
	opt.Config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	if opt.Config.UserAgent == "" {
		opt.Config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	dc, err := discovery.NewDiscoveryClientForConfig(opt.Config)
	if err != nil {
		return err
	}

	return opt.dumpAPIResources(dc)
}

func (opt *namespaceBackupManager) dumpAPIResources(dc discovery.DiscoveryInterface) error {
	resList, err := dc.ServerPreferredResources()
	if err != nil {
		return err
	}

	for _, group := range resList {
		err := opt.dumpGroup(dc, group)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *namespaceBackupManager) dumpGroup(dc discovery.DiscoveryInterface, group *metav1.APIResourceList) error {
	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return err
	}


	for _, res := range group.APIResources {
		if isSubResource(res.Name) || !hasGetListVerbs(res.Verbs) {
			continue
		}
		err = opt.dumpResource(dc, res)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *namespaceBackupManager) dumpResource(dis discovery.DiscoveryInterface, res metav1.APIResource) error {
	var ri dynamic.ResourceInterface
	if res.Namespaced{
		ri =
	}
}

func (opt *namespaceBackupManager)dumpResourceInstances(dc dynamic.Interface,gvr schema.GroupVersionResource) error{
	var next string
	for {
		var ri dynamic.ResourceInterface
		if !opt.AllNamespaces{
			ri = dc.Resource(gvr).Namespace(opt.Namespace)
		}else{
			ri = dc.Resource(gvr)
		}

		resp,err:=ri.List(context.TODO(),metav1.ListOptions{
			Limit: 250,
			Continue: next,
		})
		if err!=nil{
			return err
		}
		err=opt.storeAsYAML("",resp.Items)
		if err!=nil{
			return err
		}

		next = resp.GetContinue()
		if next == ""{
			break
		}
	}
	return nil
}

func (opt *namespaceBackupManager)storeAsYAML(prefix string,items []unstructured.Unstructured) error  {
	for _,item:=range items{
		fileName := filepath.Join(prefix,item.GetNamespace(),item.GetName())+".yaml"
		data,err:=yaml.Marshal(item.Object)
		if err!=nil{
			return err
		}
		err = opt.Storage.Write(fileName,data)
		if err!=nil{
			return err
		}
	}
	return nil
}

func isSubResource(name string) bool {
	return strings.ContainsRune(name, '/')
}

func hasGetListVerbs(verbs []string) bool {
	return sets.NewString(verbs...).HasAll("get", "list")
}
