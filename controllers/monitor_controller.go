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
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	bazv1beta1 "github.com/samze/dynamicreconcilerwithcancel/api/v1beta1"
	v1beta1 "github.com/samze/dynamicreconcilerwithcancel/api/v1beta1"
)

// MonitorReconciler reconciles a Monitor object
type MonitorReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Mgrs   *DynamicManagerMap
}

const monitorFinalizer = "samze.monitor"

//+kubebuilder:rbac:groups=baz.samze.com,resources=monitors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=baz.samze.com,resources=monitors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=baz.samze.com,resources=monitors/finalizers,verbs=update

func (r *MonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log.WithValues("monitor", req.NamespacedName)
	l.Info("reconciling")

	monitor := &v1beta1.Monitor{}

	if err := r.Get(ctx, req.NamespacedName, monitor); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	gvk := schema.GroupVersionKind{Group: monitor.Spec.Group, Version: monitor.Spec.Version, Kind: monitor.Spec.Kind}

	if monitor.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(monitor.GetFinalizers(), monitorFinalizer) {
			controllerutil.AddFinalizer(monitor, monitorFinalizer)
			if err := r.Update(ctx, monitor); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(monitor.GetFinalizers(), monitorFinalizer) {
			if dynamicMgr := r.Mgrs.Get(gvk.Group); dynamicMgr != nil {
				l.Info("Cancelling ctx")
				dynamicMgr.Cancel()

				controllerutil.RemoveFinalizer(monitor, monitorFinalizer)
				if err := r.Update(ctx, monitor); err != nil {
					return ctrl.Result{}, err
				}

				r.Mgrs.Delete(gvk.Group)
			}
			return ctrl.Result{}, nil
		}
	}

	if dynamicMgr := r.Mgrs.Get(gvk.Group); dynamicMgr == nil {
		apiVersion, kind := gvk.ToAPIVersionAndKind()
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(apiVersion)
		obj.SetKind(kind)

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             r.Scheme,
			MetricsBindAddress: ":",
			LeaderElection:     false,
		})

		if err != nil {
			return ctrl.Result{}, err
		}

		reconciler := DynamicReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			GVK:    gvk,
			Log:    l,
		}

		err = ctrl.NewControllerManagedBy(mgr).
			For(obj).
			Complete(&reconciler)

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			if err := mgr.Start(ctx); err != nil {
				l.Error(err, "Manager exited")
			}
		}()

		r.Mgrs.Put(gvk.Group, &DynamicManager{
			Mgr:    mgr,
			Cancel: cancel,
		})

		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bazv1beta1.Monitor{}).
		Complete(r)
}

type DynamicReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	GVK    schema.GroupVersionKind
}

func (d *DynamicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := d.Log.WithValues("dynamic", req.NamespacedName)
	l.Info("reconciling dynamic")

	return ctrl.Result{}, nil
}

type DynamicManagerMap struct {
	Mgrs map[string]*DynamicManager
	lock sync.RWMutex
}

type DynamicManager struct {
	Mgr    ctrl.Manager
	Cancel context.CancelFunc
}

func NewDynamicManagerMap() *DynamicManagerMap {
	return &DynamicManagerMap{
		Mgrs: make(map[string]*DynamicManager),
		lock: sync.RWMutex{},
	}
}

func (d *DynamicManagerMap) Get(key string) *DynamicManager {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.Mgrs[key]
}

func (d *DynamicManagerMap) Put(key string, mgr *DynamicManager) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.Mgrs[key] = mgr
}

func (d *DynamicManagerMap) Delete(key string) {
	d.lock.Lock()
	defer d.lock.Unlock()
	delete(d.Mgrs, key)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
