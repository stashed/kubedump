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

type metadataSanitizer struct{}

func newMetadataSanitizer() Sanitizer {
	return metadataSanitizer{}
}

func (s metadataSanitizer) Sanitize(in map[string]any) (map[string]any, error) {
	meta, ok := in["metadata"].(map[string]any)
	if !ok {
		return in, nil
	}

	delete(meta, "creationTimestamp")
	delete(meta, "resourceVersion")
	delete(meta, "uid")
	delete(meta, "generateName")
	delete(meta, "generation")

	annotations, ok := meta["annotations"]
	if ok {
		meta["annotations"] = cleanUpAnnotations(annotations)
	}
	delete(meta, "managedFields")

	in["metadata"] = meta
	return in, nil
}

func cleanUpAnnotations(in any) any {
	m, ok := in.(map[string]string)
	if !ok {
		return in
	}
	delete(m, "controller-uid")
	delete(m, "deployment.kubernetes.io/desired-replicas")
	delete(m, "deployment.kubernetes.io/max-replicas")
	delete(m, "deployment.kubernetes.io/revision")
	delete(m, "pod-template-hash")
	delete(m, "pv.kubernetes.io/bind-completed")
	delete(m, "pv.kubernetes.io/bound-by-controller")
	return m
}
