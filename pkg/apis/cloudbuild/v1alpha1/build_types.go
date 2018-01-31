/*
Copyright 2018 Google, Inc. All rights reserved.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Build is a specification for a Build resource
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildSpec   `json:"spec"`
	Status BuildStatus `json:"status"`
}

// BuildSpec is the spec for a Build resource
type BuildSpec struct {
	Source *SourceSpec        `json:"source,omitempty"`
	Steps  []corev1.Container `json:"steps,omitempty"`

	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Template, if specified,references a BuildTemplate resource to use to
	// populate fields in the build, and optional Arguments to pass to the
	// template.
	Template *TemplateInstantiationSpec `json:"template,omitempty"`
}

type TemplateInstantiationSpec struct {
	// Name references the BuildTemplate resource to use.
	Name string `json:"name"`

	// Namespace, if specified, is the namespace of the BuildTemplate resource to
	// use. If omitted, the build's namespace is used.
	Namespace string `json:"namespace,omitempty"`

	// Arguments, if specified, lists values that should be applied to the
	// parameters specified by the template.
	Arguments []ArgumentSpec `json:"arguments,omitempty"`
}

// ArgumentSpec defines the actual values to use to populate a template's
// parameters.
type ArgumentSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// TODO(jasonhall): ValueFrom?
}

// SourceSpec defines the input to the Build
type SourceSpec struct {
	Git    *GitSourceSpec    `json:"git,omitempty"`
	Custom *corev1.Container `json:"custom,omitempty"`
}

type GitSourceSpec struct {
	Url string `json:"url"`

	// One of these may be specified.
	Branch string `json:"branch,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Ref    string `json:"ref,omitempty"`
	Commit string `json:"commit,omitempty"`

	// TODO(mattmoor): authn/z
}

type BuildProvider string

const (
	// GoogleBuildProvider indicates that this build was performed with Google Container Builder.
	GoogleBuildProvider BuildProvider = "Google"
	// ClusterBuildProvider indicates that this build was performed on-cluster.
	ClusterBuildProvider BuildProvider = "Cluster"
)

// BuildStatus is the status for a Build resource
type BuildStatus struct {
	Builder BuildProvider `json:"builder,omitempty"`

	// Additional information based on the Builder executing this build.
	Cluster *ClusterSpec `json:"cluster,omitempty"`
	Google  *GoogleSpec  `json:"google,omitempty"`

	// Information about the execution of the build.
	StartTime      metav1.Time `json:"startTime,omitEmpty"`
	CompletionTime metav1.Time `json:"completionTime,omitEmpty"`

	Conditions []BuildCondition `json:"conditions,omitempty"`
}

type ClusterSpec struct {
	Namespace string `json:"namespace"`
	JobName   string `json:"jobName"`
}

type GoogleSpec struct {
	Operation string `json:"operation"`
}

type BuildConditionType string

const (
	// BuildComplete specifies that the build has completed successfully.
	BuildComplete BuildConditionType = "Complete"
	// BuildFailed specifies that the build has failed.
	BuildFailed BuildConditionType = "Failed"
	// BuildInvalid specifies that the given build specification is invalid.
	BuildInvalid BuildConditionType = "Invalid"
)

// BuildCondition defines a readiness condition for a Build.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type BuildCondition struct {
	Type BuildConditionType `json:"state"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" description:"last time the condition transit from one status to another"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildList is a list of Build resources
type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Build `json:"items"`
}
