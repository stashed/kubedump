package manager

import (
	"context"
	"path/filepath"
	"strings"

	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/manifest-backup/pkg/manager/sanitizers"

	"gomodules.xyz/sets"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type genericResourceDumper struct {
	disc      discovery.DiscoveryInterface
	di        dynamic.Interface
	namespace string
	storage   Writer
	config    *rest.Config
	sanitize  bool
	dataDir   string
}

func newGenericResourceDumper(opt BackupOptions) BackupManager {
	mgr := genericResourceDumper{
		config:   opt.Config,
		storage:  opt.Storage,
		sanitize: opt.Sanitize,
		dataDir:  opt.DataDir,
	}
	if opt.Target.Kind == apis.KindNamespace {
		mgr.namespace = opt.Target.Name
	}
	return mgr
}

func (opt genericResourceDumper) Dump() error {
	var err error
	opt.config.QPS = 1e6
	opt.config.Burst = 1e6
	if err := rest.SetKubernetesDefaults(opt.config); err != nil {
		return err
	}
	opt.config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	if opt.config.UserAgent == "" {
		opt.config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	opt.disc, err = discovery.NewDiscoveryClientForConfig(opt.config)
	if err != nil {
		return err
	}

	opt.di, err = dynamic.NewForConfig(opt.config)
	if err != nil {
		return err
	}

	return opt.dumpAPIResources()
}

func (opt *genericResourceDumper) dumpAPIResources() error {
	resList, err := opt.disc.ServerPreferredResources()
	if err != nil {
		return err
	}

	for _, group := range resList {
		err := opt.dumpGroup(group)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *genericResourceDumper) dumpGroup(group *metav1.APIResourceList) error {
	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return err
	}

	for _, res := range group.APIResources {
		if isSubResource(res.Name) || !hasGetListVerbs(res.Verbs) {
			continue
		}
		// don't process non-namespaced resources when target is a namespace
		if !res.Namespaced && opt.namespace != "" {
			continue
		}

		err := opt.dumpResourceInstances(gv.WithResource(res.Name), res.Namespaced)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *genericResourceDumper) dumpResourceInstances(gvr schema.GroupVersionResource, isNamespaced bool) error {
	klog.V(5).Infof("Dumping: ", gvr.String())
	var next string
	for {
		var ri dynamic.ResourceInterface
		if opt.namespace != "" {
			ri = opt.di.Resource(gvr).Namespace(opt.namespace)
		} else {
			ri = opt.di.Resource(gvr)
		}

		resp, err := ri.List(context.TODO(), metav1.ListOptions{
			Limit:    250,
			Continue: next,
		})
		if err != nil {
			if !kerr.IsNotFound(err) {
				return err
			}
			return nil
		}

		err = opt.processItems(resp.Items, isNamespaced)
		if err != nil {
			return err
		}

		next = resp.GetContinue()
		if next == "" {
			break
		}
	}
	return nil
}

func (opt *genericResourceDumper) processItems(items []unstructured.Unstructured, isNamespaced bool) error {
	var err error
	for _, r := range items {
		data := r.Object
		if opt.sanitize {
			s := sanitizers.NewSanitizer(r.GetKind())
			data, err = s.Sanitize(data)
			if err != nil {
				return err
			}
			delete(data, "status")
		}

		fileName := opt.getFileName(r, isNamespaced)
		err = opt.storeItem(fileName, data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *genericResourceDumper) getFileName(r unstructured.Unstructured, isNamespaced bool) string {
	prefix := ""
	if isNamespaced {
		prefix = filepath.Join(opt.dataDir, "namespaces", r.GetNamespace())
	} else {
		prefix = filepath.Join(opt.dataDir, "global")
	}
	return filepath.Join(prefix, r.GetKind(), r.GetName()) + ".yaml"
}

func (opt *genericResourceDumper) storeItem(fileName string, in map[string]interface{}) error {
	data, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	err = opt.storage.Write(fileName, data)
	if err != nil {
		return err
	}
	return nil
}

func isSubResource(name string) bool {
	return strings.ContainsRune(name, '/')
}

func hasGetListVerbs(verbs []string) bool {
	return sets.NewString(verbs...).HasAll("get", "list")
}
