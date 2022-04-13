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

import "fmt"

type workloadSanitizer struct{}

func newWorkloadSanitizer() Sanitizer {
	return workloadSanitizer{}
}

func (s workloadSanitizer) Sanitize(in map[string]interface{}) (map[string]interface{}, error) {
	ms := newMetadataSanitizer()
	in, err := ms.Sanitize(in)
	if err != nil {
		return nil, err
	}

	spec, ok := in["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to parse workload spec")
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to parse pod template")
	}
	ps := newPodSanitizer()
	template, err = ps.Sanitize(template)
	if err != nil {
		return nil, err
	}
	spec["template"] = template
	in["spec"] = spec
	return in, nil
}
