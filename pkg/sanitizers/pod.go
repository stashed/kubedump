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

package sanitizers

import (
	"fmt"

	core "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type podSanitizer struct{}

func newPodSanitizer() Sanitizer {
	return podSanitizer{}
}

func (s podSanitizer) Sanitize(in map[string]interface{}) (map[string]interface{}, error) {
	ms := newMetadataSanitizer()
	in, err := ms.Sanitize(in)
	if err != nil {
		return nil, err
	}
	spec, ok := in["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid pod spec")
	}
	spec, err = cleanUpPodSpec(spec)
	if err != nil {
		return nil, err
	}
	in["spec"] = spec
	return in, nil
}

func cleanUpPodSpec(in map[string]interface{}) (map[string]interface{}, error) {
	b, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	spec := &core.PodSpec{}
	err = yaml.Unmarshal(b, spec)
	if err != nil {
		return in, nil // Not a podSpec
	}
	spec.DNSPolicy = core.DNSPolicy("")
	spec.NodeName = ""
	if spec.ServiceAccountName == "default" {
		spec.ServiceAccountName = ""
	}
	spec.TerminationGracePeriodSeconds = nil
	for i, c := range spec.Containers {
		c.TerminationMessagePath = ""
		spec.Containers[i] = c
	}
	for i, c := range spec.InitContainers {
		c.TerminationMessagePath = ""
		spec.InitContainers[i] = c
	}
	b, err = yaml.Marshal(spec)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	err = yaml.Unmarshal(b, &out)
	return out, err
}
