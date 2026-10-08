/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	cachev1 "x1kun.com/simplecache-operator/api/v1"
)

// SimpleCacheReconciler reconciles a SimpleCache object
type SimpleCacheReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const peerListEnv = "PEERS"

// +kubebuilder:rbac:groups=cache.x1kun.com,resources=simplecaches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.x1kun.com,resources=simplecaches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.x1kun.com,resources=simplecaches/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SimpleCache object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile
func (r *SimpleCacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Initialize the request-scoped logger.
	logger := log.FromContext(ctx)

	// Fetch the SimpleCache resource from the API server.
	var cacheResource cachev1.SimpleCache
	if err := r.Get(ctx, req.NamespacedName, &cacheResource); err != nil {
		// Ignore a deleted or missing resource.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Record the desired cluster configuration.
	logger.Info("Reconciling SimpleCache",
		"clusterName", cacheResource.Name,
		"desiredReplicas", cacheResource.Spec.Size,
		"image", cacheResource.Spec.Image,
	)

	// Build the static peer list from the desired replica count.
	// For three replicas, use the stable Pod DNS names for ordinals 0 through 2.
	var peers []string
	svcName := cacheResource.Name + "-svc" // Name of the governing headless Service.
	for i := int32(0); i < cacheResource.Spec.Size; i++ {
		peerURL := fmt.Sprintf("http://%s-%d.%s.%s.svc.cluster.local:8001",
			cacheResource.Name, i, svcName, cacheResource.Namespace)
		peers = append(peers, peerURL)
	}
	peersStr := strings.Join(peers, ",")

	labels := map[string]string{"app": cacheResource.Name}

	// Construct the desired headless Service.
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: cacheResource.Namespace,
		},
		Spec: corev1.ServiceSpec{
			// Headless Service.
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{Name: "peer", Port: 8001},
				{Name: "api", Port: 9999},
			},
		},
	}
	// Assign the SimpleCache resource as the Service owner.
	if err := ctrl.SetControllerReference(&cacheResource, svc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Create the governing Service if it does not exist.
	foundSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, foundSvc)
	if err != nil && apierrors.IsNotFound(err) {
		logger.Info("Creating headless Service", "Name", svc.Name)
		if err = r.Create(ctx, svc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Construct the desired StatefulSet.
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cacheResource.Name,
			Namespace: cacheResource.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &cacheResource.Spec.Size,
			// Use the governing headless Service.
			ServiceName: svcName,
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "cache",
						Image:   cacheResource.Spec.Image,
						Command: []string{"./geecache-server", "-port=8001", "-api=1"},
						Env: []corev1.EnvVar{
							{
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
								},
							},
							{
								Name:  "SELF_ADDR",
								Value: fmt.Sprintf("http://$(POD_NAME).%s.%s.svc.cluster.local:8001", svcName, cacheResource.Namespace),
							},
							{
								Name:  peerListEnv,
								Value: peersStr, // Static peer list derived from the replica count.
							},
						},
					}},
				},
			},
		},
	}

	// Assign ownership so Kubernetes can garbage-collect the StatefulSet when the CR is deleted.
	if err = ctrl.SetControllerReference(&cacheResource, sts, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Look up the current StatefulSet.
	foundSts := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, foundSts)

	if err != nil && apierrors.IsNotFound(err) {
		// Create the StatefulSet when missing.
		logger.Info("Creating StatefulSet", "Name", sts.Name)
		err = r.Create(ctx, sts)
		if err != nil {
			return ctrl.Result{}, err
		}
		// Creation completed.
		return ctrl.Result{}, nil
	} else if err != nil {
		// Propagate API or transport errors.
		return ctrl.Result{}, err
	}

	var existingPeers string
	for _, env := range foundSts.Spec.Template.Spec.Containers[0].Env {
		if env.Name == peerListEnv {
			existingPeers = env.Value
			break
		}
	}
	existingImage := foundSts.Spec.Template.Spec.Containers[0].Image

	// Compare the current StatefulSet with the desired configuration.
	if *foundSts.Spec.Replicas != cacheResource.Spec.Size || existingPeers != peersStr || existingImage != cacheResource.Spec.Image {
		logger.Info("Updating StatefulSet configuration", "currentImage", existingImage, "desiredImage", cacheResource.Spec.Image)
		// Update the replica count.
		foundSts.Spec.Replicas = &cacheResource.Spec.Size
		// Update the cache image.
		foundSts.Spec.Template.Spec.Containers[0].Image = cacheResource.Spec.Image
		// Update the static PEERS environment variable.
		for i, env := range foundSts.Spec.Template.Spec.Containers[0].Env {
			if env.Name == peerListEnv {
				// Use the freshly generated peer list.
				foundSts.Spec.Template.Spec.Containers[0].Env[i].Value = peersStr
				break
			}
		}
		err = r.Update(ctx, foundSts)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	// Wait for the next reconciliation event.
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimpleCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1.SimpleCache{}).
		Named("simplecache").
		Complete(r)
}
