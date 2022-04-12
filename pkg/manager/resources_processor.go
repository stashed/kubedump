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
	config        *rest.Config
	disc          discovery.DiscoveryInterface
	di            dynamic.Interface
	itemProcessor itemProcessor
	namespace     string
	selector      string
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

		err := opt.processResourceInstances(gv.WithResource(res.Name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *resourceProcessor) processResourceInstances(gvr schema.GroupVersionResource) error {
	klog.V(5).Infof("Processing: ", gvr.String())
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
