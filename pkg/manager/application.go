package manager

import (
	"context"
	"path/filepath"

	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	"stash.appscode.dev/manifest-backup/pkg/sanitizers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type applicationBackupManager struct {
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
	gvr, err := opt.getRootObjectGVR()
	if err != nil {
		return nil
	}
	rootObj, err := opt.getRootObject(*gvr)
	if err != nil {
		return err
	}

	rTree := treeBuilder{
		resourceTree: make(map[types.UID][]resourceRef),
	}
	rootUID := types.UID("root")
	rTree.resourceTree[rootUID] = []resourceRef{
		{
			gvr:       *gvr,
			name:      rootObj.GetName(),
			namespace: rootObj.GetNamespace(),
			kind:      rootObj.GetKind(),
		},
	}

	if opt.includeDependants {
		err := opt.generateDependencyTree(&rTree)
		if err != nil {
			return err
		}
	}
	return opt.dumpResourceTree(rTree.resourceTree, rootUID, opt.dataDir)
}

func (opt *applicationBackupManager) getRootObjectGVR() (*schema.GroupVersionResource, error) {
	disc, err := discovery.NewDiscoveryClientForConfig(opt.config)
	if err != nil {
		return nil, err
	}
	apiResources, err := restmapper.GetAPIGroupResources(disc)
	if err != nil {
		return nil, err
	}

	gv, err := schema.ParseGroupVersion(opt.target.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := gv.WithKind(opt.target.Kind)

	mapper := restmapper.NewDiscoveryRESTMapper(apiResources)
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		return nil, err
	}
	return &mapping.Resource, nil
}

func (opt *applicationBackupManager) getRootObject(gvr schema.GroupVersionResource) (*unstructured.Unstructured, error) {
	var err error
	opt.di, err = dynamic.NewForConfig(opt.config)
	if err != nil {
		return nil, err
	}
	ri := opt.di.Resource(gvr).Namespace(opt.namespace)
	return ri.Get(context.TODO(), opt.target.Name, metav1.GetOptions{})
}

func (opt *applicationBackupManager) generateDependencyTree(tb *treeBuilder) error {
	rp := resourceProcessor{
		config:        opt.config,
		namespace:     opt.namespace,
		selector:      opt.selector,
		itemProcessor: tb,
	}
	return rp.processAPIResources()
}

type resourceRef struct {
	gvr       schema.GroupVersionResource
	name      string
	namespace string
	kind      string
}

type treeBuilder struct {
	resourceTree map[types.UID][]resourceRef
}

func (opt treeBuilder) Process(items []unstructured.Unstructured, gvr schema.GroupVersionResource) error {
	for _, r := range items {
		ownerRefs := r.GetOwnerReferences()
		for i := range ownerRefs {
			if _, ok := opt.resourceTree[ownerRefs[i].UID]; !ok {
				opt.resourceTree[ownerRefs[i].UID] = make([]resourceRef, 0)
			}
			opt.resourceTree[ownerRefs[i].UID] = append(opt.resourceTree[ownerRefs[i].UID], resourceRef{
				gvr:       gvr,
				name:      r.GetName(),
				namespace: r.GetNamespace(),
				kind:      r.GetKind(),
			})
		}
	}
	return nil
}

func (opt *applicationBackupManager) dumpResourceTree(resourceTree map[types.UID][]resourceRef, rootUID types.UID, prefix string) error {
	for _, r := range resourceTree[rootUID] {
		childUID, err := opt.dumpItem(r, prefix)
		if err != nil {
			return err
		}
		childPrefix := prefix
		if rootUID != "root" {
			childPrefix = filepath.Join(prefix, r.kind, r.name)
		}
		err = opt.dumpResourceTree(resourceTree, childUID, childPrefix)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *applicationBackupManager) dumpItem(r resourceRef, prefix string) (types.UID, error) {
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

	fileName := opt.getFileName(obj, prefix)
	return uid, storeItem(fileName, data, opt.storage)
}

func (opt *applicationBackupManager) getFileName(r *unstructured.Unstructured, prefix string) string {
	if opt.target.Kind == r.GetKind() &&
		opt.target.Name == r.GetName() &&
		opt.namespace == r.GetNamespace() {
		return filepath.Join(prefix, r.GetName()) + ".yaml"
	}
	return filepath.Join(prefix, r.GetKind(), r.GetName(), r.GetName()) + ".yaml"
}
