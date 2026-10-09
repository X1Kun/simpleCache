package controller

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
)

// Preserve API-defaulted and unrelated Pod fields while reconciling owned fields.
func mergeTemplate(current, desired *corev1.PodTemplateSpec) {
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}
	maps.Copy(current.Labels, desired.Labels)
	if current.Annotations == nil {
		current.Annotations = map[string]string{}
	}
	maps.Copy(current.Annotations, desired.Annotations)
	current.Spec.ServiceAccountName = desired.Spec.ServiceAccountName
	current.Spec.DeprecatedServiceAccount = desired.Spec.ServiceAccountName
	current.Spec.TerminationGracePeriodSeconds = desired.Spec.TerminationGracePeriodSeconds
	wanted := desired.Spec.Containers[0]
	for i := range current.Spec.Containers {
		container := &current.Spec.Containers[i]
		if container.Name != wanted.Name {
			continue
		}
		container.Image = wanted.Image
		container.ImagePullPolicy = wanted.ImagePullPolicy
		container.Command = wanted.Command
		container.Args = wanted.Args
		container.Env = wanted.Env
		container.Ports = wanted.Ports
		container.Resources = wanted.Resources
		container.StartupProbe = wanted.StartupProbe
		container.LivenessProbe = wanted.LivenessProbe
		container.ReadinessProbe = wanted.ReadinessProbe
		return
	}
	current.Spec.Containers = append(current.Spec.Containers, wanted)
}
