/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// License defines a AppsCode product license info.
type License struct {
	metav1.TypeMeta `json:",inline,omitempty"`

	Data         []byte        `json:"-"`
	Issuer       string        `json:"issuer,omitempty"` // byte.builders
	ProductLine  string        `json:"productLine,omitempty"`
	TierName     string        `json:"tierName,omitempty"`
	PlanName     string        `json:"planName,omitempty"`
	Features     []string      `json:"features,omitempty"`
	FeatureFlags FeatureFlags  `json:"featureFlags,omitempty"`
	Clusters     []string      `json:"clusters,omitempty"` // cluster_id ?
	User         *User         `json:"user,omitempty"`
	NotBefore    *metav1.Time  `json:"notBefore,omitempty"` // start of subscription start
	NotAfter     *metav1.Time  `json:"notAfter,omitempty"`  // if set, use this
	ID           string        `json:"id,omitempty"`        // license ID
	Status       LicenseStatus `json:"status"`
	Reason       string        `json:"reason"`
}

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// +kubebuilder:validation:Enum=unknown;active;invalid;canceled
type LicenseStatus string

const (
	LicenseUnknown  LicenseStatus = "unknown"
	LicenseActive   LicenseStatus = "active"
	LicenseInvalid  LicenseStatus = "invalid"
	LicenseCanceled LicenseStatus = "canceled"
)

type Contract struct {
	ID              string      `json:"id"`
	StartTimestamp  metav1.Time `json:"startTimestamp"`
	ExpiryTimestamp metav1.Time `json:"expiryTimestamp"`
}

type FeatureFlag string

const (
	FeatureDisableAnalytics    FeatureFlag = "DisableAnalytics"
	FeatureRestrictions        FeatureFlag = "Restrictions"
	FeatureEnableClientBilling FeatureFlag = "EnableClientBilling"
)

var knownFlags = sets.New[FeatureFlag](FeatureDisableAnalytics, FeatureRestrictions, FeatureEnableClientBilling)

type FeatureFlags map[FeatureFlag]string

func (f FeatureFlags) IsValid() error {
	var errs []error
	for k := range f {
		if !knownFlags.Has(k) {
			errs = append(errs, fmt.Errorf("unknown feature flag %q", k))
		}
	}
	return errors.NewAggregate(errs)
}

func (f FeatureFlags) ToSlice() []string {
	if len(f) == 0 {
		return nil
	}
	result := make([]string, 0, len(f))
	for k, v := range f {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}
