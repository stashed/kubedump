package manager

import (
	"context"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"path/filepath"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	"stash.appscode.dev/manifest-backup/pkg/manager/sanitizers"
)

type applicationBackupManager struct {
	disc              discovery.DiscoveryInterface
	di                dynamic.Interface
	namespace         string
	storage           Writer
	config            *rest.Config
	sanitize          bool
	dataDir           string
	selector          string
	includeDependants bool
	target            v1beta1.TargetRef
}

func newApplicationBackupManager(opt BackupOptions) BackupManager {
	return applicationBackupManager{
		config:            opt.Config,
		storage:           opt.Storage,
		sanitize:          opt.Sanitize,
		dataDir:           opt.DataDir,
		selector:          opt.Selector,
		namespace:         opt.Namespace,
		includeDependants: opt.IncludeDependants,
		target:            opt.Target,
	}
}

func (opt applicationBackupManager) Dump() error {
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

	gv, err := schema.ParseGroupVersion(opt.target.APIVersion)
	if err != nil {
		return err
	}
	gvk := gv.WithKind(opt.target.Kind)

	apiResources, err := restmapper.GetAPIGroupResources(opt.disc)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(apiResources)
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		return err
	}
	ri := opt.di.Resource(mapping.Resource).Namespace(opt.namespace)

	rootObj, err := ri.Get(context.TODO(), opt.target.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	resourceTree := make(map[types.UID][]resourceRef)
	rootUID := types.UID("root")
	resourceTree[rootUID] = []resourceRef{
		{
			gvr:       mapping.Resource,
			name:      rootObj.GetName(),
			namespace: rootObj.GetNamespace(),
		},
	}

	if opt.includeDependants {
		err := opt.generateDependencyTree(resourceTree)
		if err != nil {
			return err
		}
	}
	return opt.dumpResourceTree(resourceTree, rootUID)
}

type resourceRef struct {
	gvr       schema.GroupVersionResource
	name      string
	namespace string
}

func (opt *applicationBackupManager) generateDependencyTree(resourceTree map[types.UID][]resourceRef) error {
	resList, err := opt.disc.ServerPreferredResources()
	if err != nil {
		return err
	}

	for _, group := range resList {
		err := opt.traverseGroup(resourceTree, group)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *applicationBackupManager) traverseGroup(resourceTree map[types.UID][]resourceRef, group *metav1.APIResourceList) error {
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

		err := opt.traverseResourceInstances(resourceTree, gv.WithResource(res.Name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *applicationBackupManager) traverseResourceInstances(resourceTree map[types.UID][]resourceRef, gvr schema.GroupVersionResource) error {
	var next string
	for {
		var ri dynamic.ResourceInterface
		if opt.namespace != "" {
			ri = opt.di.Resource(gvr).Namespace(opt.namespace)
		} else {
			ri = opt.di.Resource(gvr)
		}

		resp, err := ri.List(context.TODO(), metav1.ListOptions{
			Limit:         250,
			Continue:      next,
			LabelSelector: opt.selector,
		})
		if err != nil {
			if !kerr.IsNotFound(err) {
				return err
			}
			return nil
		}

		opt.processItems(resourceTree, resp.Items, gvr)

		next = resp.GetContinue()
		if next == "" {
			break
		}
	}
	return nil
}

func (opt *applicationBackupManager) processItems(resourceTree map[types.UID][]resourceRef, items []unstructured.Unstructured, gvr schema.GroupVersionResource) {
	for _, r := range items {
		ownerRefs := r.GetOwnerReferences()
		for i := range ownerRefs {
			if _, ok := resourceTree[ownerRefs[i].UID]; !ok {
				resourceTree[ownerRefs[i].UID] = make([]resourceRef, 0)
			}
			resourceTree[ownerRefs[i].UID] = append(resourceTree[ownerRefs[i].UID], resourceRef{
				gvr:       gvr,
				name:      r.GetName(),
				namespace: r.GetNamespace(),
			})
		}
	}
}

func (opt *applicationBackupManager) dumpResourceTree(resourceTree map[types.UID][]resourceRef, rootUID types.UID) error {
	for _, r := range resourceTree[rootUID] {
		childUID, err := opt.dumpItem(r)
		if err != nil {
			return err
		}
		err = opt.dumpResourceTree(resourceTree, childUID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *applicationBackupManager) dumpItem(r resourceRef) (types.UID, error) {
	var ri dynamic.ResourceInterface
	if r.namespace != "" {
		ri = opt.di.Resource(r.gvr).Namespace(r.namespace)
	} else {
		ri = opt.di.Resource(r.gvr)
	}

	obj, err := ri.Get(context.TODO(), r.name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	data := obj.Object
	uid := obj.GetUID()
	if opt.sanitize {
		s := sanitizers.NewSanitizer(obj.GetKind())
		data, err = s.Sanitize(data)
		if err != nil {
			return "", err
		}
		delete(data, "status")
	}

	fileName := getFileName(obj, true, opt.dataDir)
	return uid, storeItem(fileName, data, opt.storage)
}

func getFileName(r *unstructured.Unstructured, isNamespaced bool, dataDir string) string {
	prefix := ""
	if isNamespaced {
		prefix = filepath.Join(dataDir, "namespaces", r.GetNamespace())
	} else {
		prefix = filepath.Join(dataDir, "global")
	}
	return filepath.Join(prefix, r.GetKind(), r.GetName()) + ".yaml"
}
