/*
Copyright 2021.

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

package controllers

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hostapd "github.com/RyuSA/accesspoint-operator/internal/hostapd"

	accesspointv1alpha1 "github.com/RyuSA/accesspoint-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

var (
	controllerOwnerKey = ".metadata.controller"
	apiGVStr           = accesspointv1alpha1.GroupVersion.String()
)

// AccessPointReconciler reconciles a AccessPoint object
type AccessPointReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=accesspoint.ryusa.github.com,resources=accesspoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=accesspoint.ryusa.github.com,resources=accesspoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=accesspoint.ryusa.github.com,resources=accesspoints/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AccessPoint object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *AccessPointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// your logic here

	log := r.Log.WithValues("accesspoint", req.NamespacedName)
	log.Info("start reconsile")

	accesspoint_base_label := map[string]string{
		"app.kubernetes.io/name":     "accesspoint",
		"app.kubernetes.io/instance": req.Name,
	}

	// リクエスト内容を元にAPを検索します
	// このAPと1対1に紐つくDaemonSetを作成していきます
	var accesspoint accesspointv1alpha1.AccessPoint
	log.Info("fetching AccessPoint named", "name", req.NamespacedName)
	if err := r.Get(ctx, req.NamespacedName, &accesspoint); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Accesspoint has been successfully fetched")
	hostapdConfig := NewHostapdConfiguration(&accesspoint)

	// APの組み合わせに紐つくDaemonSetを取得します
	// 存在していれば変更点の調査、更新します
	// 存在していなければ新規作成します
	log.Info("fetching related DaemonSets...")
	var daemonsets appsv1.DaemonSetList
	if err := r.List(ctx, &daemonsets, client.MatchingLabels(accesspoint_base_label)); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("DaemonSets has been successfully fetched")

	// APに紐つくDaemonSetの個数
	numberOfDaemonSetRelatedToAp := len(daemonsets.Items)

	createLabel := func(label map[string]string, instance string) map[string]string {
		if label == nil {
			label = make(map[string]string)
		}
		label["app.kubernetes.io/name"] = "accesspoint"
		label["app.kubernetes.io/instance"] = instance
		return label
	}

	// TODO ましな実装にする
	if numberOfDaemonSetRelatedToAp == 0 || numberOfDaemonSetRelatedToAp == 1 {
		log.Info("creating configmap...")
		configmap := &corev1.ConfigMap{}
		configmap.SetName(req.Name)
		configmap.SetNamespace(req.Namespace)
		if _, err := ctrl.CreateOrUpdate(ctx, r.Client, configmap, func() error {
			configmap.Labels = createLabel(configmap.Labels, req.Name)
			configmap.Data = make(map[string]string)
			configmap.Data["hostapd.conf"] = hostapdConfig.Configure()
			if err := ctrl.SetControllerReference(&accesspoint, configmap, r.Scheme); err != nil {
				log.Error(err, "unable to set ownerReference")
				return err
			}
			return nil
		}); err != nil {
			log.Error(err, "Error during creating configmap...")
			return ctrl.Result{
				RequeueAfter: time.Minute * 1,
			}, err
		}
		log.Info("configmap has been successfully created")

		log.Info("creating daemonset...")
		daemonset := &appsv1.DaemonSet{}
		daemonset.SetName(req.Name)
		daemonset.SetNamespace(req.Namespace)
		if _, err := ctrl.CreateOrUpdate(ctx, r.Client, daemonset, func() error {
			daemonset.Labels = createLabel(daemonset.Labels, req.Name)
			daemonset.Spec = appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: accesspoint_base_label,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: accesspoint_base_label,
					},
					Spec: corev1.PodSpec{
						HostNetwork: true,
						Containers: []corev1.Container{
							{
								Image: "ryusa/simple-accesspoint:latest",
								Name:  "accesspoint",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config",
										MountPath: "/hostapd/conf/",
									},
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: &[]bool{true}[0],
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: req.Name,
										},
									},
								},
							},
						},
						NodeSelector: accesspoint.Spec.NodeSelector,
					},
				},
			}

			if err := ctrl.SetControllerReference(&accesspoint, daemonset, r.Scheme); err != nil {
				log.Error(err, "unable to set ownerReference")
				return err
			}
			return nil
		}); err != nil {
			log.Error(err, "Error during creating daemonset...")
			return ctrl.Result{
				RequeueAfter: time.Minute * 1,
			}, err
		}

		log.Info("daemonset has been successfully created")
	} else {
		log.Info("there are multiple daemonset related to this event!", "namespacedname", req.NamespacedName, "daemonsets", daemonsets)
	}

	log.Info("sync done")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessPointReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&appsv1.DaemonSet{},
		controllerOwnerKey,
		func(raw client.Object) []string {
			daemonset := raw.(*appsv1.DaemonSet)
			owner := metav1.GetControllerOf(daemonset)
			if owner == nil {
				return nil
			}
			if owner.APIVersion != apiGVStr || owner.Kind != "AccessPoint" {
				return nil
			}
			return []string{owner.Name}
		}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.ConfigMap{},
		controllerOwnerKey,
		func(raw client.Object) []string {
			configmap := raw.(*corev1.ConfigMap)
			owner := metav1.GetControllerOf(configmap)
			if owner == nil {
				return nil
			}
			if owner.APIVersion != apiGVStr || owner.Kind != "AccessPoint" {
				return nil
			}
			return []string{owner.Name}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&accesspointv1alpha1.AccessPoint{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func NewHostapdConfiguration(accesspoint *accesspointv1alpha1.AccessPoint) *hostapd.HostapdConfiguration {
	return &hostapd.HostapdConfiguration{
		Networkinterface: accesspoint.Spec.Interface,
		Ssid:             accesspoint.Spec.Ssid,
		Password:         accesspoint.Spec.Password,
	}
}
