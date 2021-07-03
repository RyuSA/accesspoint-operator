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
	"crypto/md5"
	"encoding/hex"
	"time"

	hostapd "github.com/RyuSA/accesspoint-operator/internal/hostapd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
//+kubebuilder:rbac:groups=accesspoint.ryusa.github.com,resources=accesspointdevices,verbs=get;list;watch

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
	namespace := req.Namespace
	name := req.Name
	namespacedName := req.NamespacedName

	log := r.Log.WithValues("accesspoint", namespacedName)
	log.Info("start reconsile", "request", namespacedName)

	// リクエスト内容を元にAPを検索します
	var accesspoint accesspointv1alpha1.AccessPoint
	log.Info("fetching AccessPoint", "name", namespacedName)
	if err := r.Get(ctx, namespacedName, &accesspoint); err != nil {
		log.Info("AccessPoint does not found")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("AccessPoint has been successfully fetched")

	// accesspoint.Spec.Devicesに紐つくDevice一覧を取得し配列に詰め込む
	var accaccesspointdevices []accesspointv1alpha1.AccessPointDevice
	for _, deviceName := range accesspoint.Spec.Devices {
		var accaccesspointdevice accesspointv1alpha1.AccessPointDevice
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: namespace,
			Name:      deviceName,
		}, &accaccesspointdevice); err != nil {
			log.Info("AccessPointDevice not found")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		accaccesspointdevices = append(accaccesspointdevices, accaccesspointdevice)
	}
	log.Info("AccessPointDevices have been successfully fetched")

	// 各AccessPointDeviceごとにDaemonSetとConfigMapを1つずつ紐つけます
	// ==== 基本戦略 ====
	// 1. 入力を元にhostapdの設定を作成する
	//   - AccessPointからSSIDなどのAP情報
	//   - AccessPointDeviceからインタフェースなどの物理情報
	// 2. hostapの設定からハッシュ値を取り出す
	// 3. hostapdの設定ファイルを持つConfigMapを生成
	//   - この際に"hostapdversion=${hash}"のアノテーションを付与する
	// 4. 3.で作成したConfigMapをマウントするDaemonSetを作成
	for _, device := range accaccesspointdevices {
		// ConfigMapとDaemonSetの名前
		fullName := name + "-" + device.Name

		// ラベル生成
		createLabel := func(label map[string]string, instance string) map[string]string {
			if label == nil {
				label = make(map[string]string)
			}
			label["app.kubernetes.io/name"] = "accesspoint"
			label["app.kubernetes.io/instance"] = instance
			return label
		}
		accesspoint_base_label := createLabel(make(map[string]string), fullName)

		// アノテーション生成
		createAnnotation := func(annotation map[string]string, version string) map[string]string {
			if annotation == nil {
				annotation = make(map[string]string)
			}
			annotation["hostapdversion"] = version
			return annotation
		}

		// AP/Deviceの組み合わせに紐つくDaemonSetを取得します
		log.Info("fetching related DaemonSets...")
		var daemonsets appsv1.DaemonSetList
		if err := r.List(ctx, &daemonsets, client.MatchingLabels(accesspoint_base_label)); err != nil {
			log.Info("DaemonSets do not found")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.Info("DaemonSets have been successfully fetched")

		// APに紐つくDaemonSetの個数
		theNumberOfDaemonSetRelatedToAp := len(daemonsets.Items)

		// DaemonSetの数が適正、つまり作成か更新の対象になっているかどうか
		// DaemonSetの数 = 0: まだ同期していないのでDaemonSetを作成する
		// DaemonSetの数 = 1: 同期済みのDaemonSetあり、更新する
		theNumberOfDaemonSetIsExpected := theNumberOfDaemonSetRelatedToAp == 0 || theNumberOfDaemonSetRelatedToAp == 1

		// なおConfigMapが2つ以上ないことを検査しないのは、2つ以上あってもDaemonSetにマウントされて
		// hostapdが読み取るのは1つまでなので検査していません
		// DaemonSetが多くとも1つ存在する場合、CreateOrUpdateで更新します
		// 2つ以上存在する場合、ユーザーがなんらか作成した可能性が高いので放置、ログに吐いてReEnqueする
		if theNumberOfDaemonSetIsExpected {
			hostapdConfig := NewHostapdConfiguration(&accesspoint, &device)
			hash := func(config string) string {
				r := md5.Sum([]byte(config))
				return hex.EncodeToString(r[:])
			}
			configString := hostapd.ConfigureHostapd(*hostapdConfig)
			configHash := hash(configString)
			log.Info("", "config", configString)
			log.Info("", "hash value", configHash)

			log.Info("creating configmap...")
			configmap := &corev1.ConfigMap{}
			configmap.SetName(fullName)
			configmap.SetNamespace(namespace)

			if _, err := ctrl.CreateOrUpdate(ctx, r.Client, configmap, func() error {
				configmap.Labels = createLabel(configmap.Labels, fullName)
				configmap.Annotations = createAnnotation(configmap.Annotations, configHash)
				configmap.Data = make(map[string]string)
				configmap.Data["hostapd.conf"] = configString
				if err := ctrl.SetControllerReference(&accesspoint, configmap, r.Scheme); err != nil {
					log.Error(err, "unable to set ownerReference")
					return err
				}
				return nil
			}); err != nil {
				log.Error(err, "Error during creating configmap...")
				return ctrl.Result{
					RequeueAfter: time.Minute,
				}, err
			}
			log.Info("configmap has been successfully created")

			log.Info("creating daemonset...")
			daemonset := &appsv1.DaemonSet{}
			daemonset.SetName(fullName)
			daemonset.SetNamespace(namespace)
			if _, err := ctrl.CreateOrUpdate(ctx, r.Client, daemonset, func() error {
				// hostapdのバージョンが完全に一致している場合更新する必要がないためreturnで抜ける
				// TODO 無理やり実装なのでうまく抜ける方法を実装する
				if daemonset.Annotations["hostapdversion"] == configHash {
					return &DuplicateHostapd{}
				}
				daemonset.Labels = createLabel(daemonset.Labels, fullName)
				daemonset.Annotations = createAnnotation(daemonset.Annotations, configHash)
				daemonset.Spec = daemonSetSpecTemplate(
					accesspoint_base_label,                              // Selector
					accesspoint_base_label,                              // PodTemplateLabel
					createAnnotation(daemonset.Annotations, configHash), // PodTemplateAnnotation
					fullName, // ConfigMapName
					device,   // AccessPointDevice
				)
				if err := ctrl.SetControllerReference(&accesspoint, daemonset, r.Scheme); err != nil {
					log.Error(err, "unable to set ownerReference")
					return err
				}
				return nil
			}); err != nil {
				// 専用エラーと一致している場合は更新せずに処理を終了する
				// TODO 無理やり実装なのでうまく実装しなおす
				dup := DuplicateHostapd{}
				if err.Error() == dup.Error() {
					log.Info("DuplicateHostapd")
					return ctrl.Result{}, nil
				}
				log.Error(err, "Error during creating daemonset...")
				return ctrl.Result{
					RequeueAfter: time.Minute,
				}, err
			}
			log.Info("daemonset has been successfully created")
		} else {
			log.Info("there are multiple daemonset related to this event!", "namespacedname", namespacedName, "daemonsets", daemonsets)
			return ctrl.Result{
				RequeueAfter: time.Minute,
			}, nil
		}
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

func NewHostapdConfiguration(accesspoint *accesspointv1alpha1.AccessPoint, accesspointdevice *accesspointv1alpha1.AccessPointDevice) *hostapd.HostapdConfiguration {
	return &hostapd.HostapdConfiguration{
		Bridge:           accesspointdevice.Spec.Bridge,
		Networkinterface: accesspointdevice.Spec.Interface,
		Password:         accesspoint.Spec.Password,
		Ssid:             accesspoint.Spec.Ssid,
	}
}

func daemonSetSpecTemplate(daemonsetLabel, podTemplateLabel, podTemplateAnnotation map[string]string, configMapName string, device accesspointv1alpha1.AccessPointDevice) appsv1.DaemonSetSpec {
	return appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: daemonsetLabel,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      podTemplateLabel,
				Annotations: podTemplateAnnotation,
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
									Name: configMapName,
								},
							},
						},
					},
				},
				NodeSelector: device.Spec.NodeSelector,
			},
		},
	}
}

type DuplicateHostapd struct{}

func (d *DuplicateHostapd) Error() string {
	return "Duplicated Hostapd"
}
