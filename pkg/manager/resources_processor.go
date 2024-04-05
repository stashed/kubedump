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
)

type resourceProcessor struct {
	config           *rest.Config
	disc             discovery.DiscoveryInterface
	di               dynamic.Interface
	itemProcessor    itemProcessor
	namespace        string
	selector         string
	ignoreGroupKinds []string
}

type itemProcessor interface {
	Process(items []unstructured.Unstructured, gvr schema.GroupVersionResource) error
}

func (opt *resourceProcessor) processAPIResources() error {
	err := opt.configure()
	if err != nil {
		return err
	}

	resList, err := opt.disc.ServerPreferredResources()
	if err != nil {
		return err
	}

	for _, group := range resList {
		err := opt.processGroup(group)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *resourceProcessor) configure() error {
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
	return nil
}

func (opt *resourceProcessor) processGroup(group *metav1.APIResourceList) error {
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

		if opt.shouldIgnoreResource(schema.GroupKind{Group: gv.WithResource(res.Name).Group, Kind: res.Kind}) {
			continue
		}

		err := opt.processResourceInstances(gv.WithResource(res.Name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *resourceProcessor) shouldIgnoreResource(gk schema.GroupKind) bool {
	for _, igk := range opt.ignoreGroupKinds {
		if gk == schema.ParseGroupKind(igk) {
			return true
		}
	}
	return false
}

func (opt *resourceProcessor) processResourceInstances(gvr schema.GroupVersionResource) error {
	klog.V(5).Infoln("Processing:", gvr)
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

		err = opt.itemProcessor.Process(resp.Items, gvr)
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
