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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	accesspointv1alpha1 "github.com/RyuSA/accesspoint-operator/api/v1alpha1"
	"github.com/go-logr/logr"
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

	// リクエスト内容を元にAPを検索します
	// このAPと1対1に紐つくDaemonSetを作成していきます
	var accesspoint accesspointv1alpha1.AccessPoint
	log.Info("fetching AccessPoint named", "name", req.NamespacedName)
	if err := r.Get(ctx, req.NamespacedName, &accesspoint); err != nil {
		// リソースが存在しない場合をここでキャプチャしています
		// リソースが存在しない = アクセスポイント設定がない ことを表現しているので対応するDaemonSetを削除します
		if apierrors.IsNotFound(err) {
			log.Info("The AP has been deleted. Deleting related DaemonSet...", "AP name", req.NamespacedName)
			// TODO 一致するDaemonSetを削除します
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to fetching Accesspoint")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("captured AccesspointDevice", "accesspoint", accesspoint)

	// APの組み合わせに紐つくDaemonSetを取得します
	// 存在していれば変更点の調査、更新します
	// 存在していなければ新規作成します
	var daemonsets appsv1.DaemonSetList
	if err := r.List(ctx, &daemonsets, client.MatchingLabels{
		"app.kubernetes.io/name":     "accesspoint",
		"app.kubernetes.io/instance": req.NamespacedName.String(),
	}); err != nil {
		log.Error(err, "failed to fetch daemonset")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("captured DaemonSet", "daemonset-list", daemonsets)

	// APに紐つくDaemonSetの個数
	numberOfDaemonSetRelatedToAp := len(daemonsets.Items)

	// TODO
	switch numberOfDaemonSetRelatedToAp {
	case 0:
		log.Info("create new DaemonSet!")
		// ConfigMapを作成します
		// DaemonSetを作成します
	case 1:
		log.Info("update the DaemonSet", "daemonset", daemonsets.Items[len(daemonsets.Items)-1])
		// ctrl.CreateOrUpdateでConfigMapを更新します
		// ctrl.CreateOrUpdateでDaemonSetを更新します
	default:
		log.Info("there are multiple daemonset related to this event!", "namespacedname", req.NamespacedName, "daemonsets", daemonsets)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessPointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&accesspointv1alpha1.AccessPoint{}).
		Complete(r)
}
