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

package manager

import (
	"context"
	"path/filepath"

	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	"stash.appscode.dev/kubedump/pkg/sanitizers"

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
	storage           Writer
	config            *rest.Config
	sanitize          bool
	dataDir           string
	selector          string
	includeDependants bool
	ignoreGroupKinds  []string
	target            v1beta1.TargetRef
}

func newApplicationBackupManager(opt BackupOptions) BackupManager {
	return applicationBackupManager{
		config:            opt.Config,
		storage:           opt.Storage,
		sanitize:          opt.Sanitize,
		dataDir:           opt.DataDir,
		selector:          opt.Selector,
		includeDependants: opt.IncludeDependants,
		ignoreGroupKinds:  opt.IgnoreGroupKinds,
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
	ri := opt.di.Resource(gvr).Namespace(opt.target.Namespace)
	return ri.Get(context.TODO(), opt.target.Name, metav1.GetOptions{})
}

func (opt *applicationBackupManager) generateDependencyTree(tb *treeBuilder) error {
	rp := resourceProcessor{
		config:           opt.config,
		namespace:        opt.target.Namespace,
		selector:         opt.selector,
		itemProcessor:    tb,
		ignoreGroupKinds: opt.ignoreGroupKinds,
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
		opt.target.Namespace == r.GetNamespace() {
		return filepath.Join(prefix, r.GetName()) + ".yaml"
	}
	return filepath.Join(prefix, r.GetKind(), r.GetName(), r.GetName()) + ".yaml"
}
