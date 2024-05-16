package operator

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/apparentlymart/go-cidr/cidr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// LoadBalancerClass allows us to have wga assign ips to pods that currently do not have one.
	// It avoids having the user set up a complicated mess of cillium/kubevip/metallb just for providing ips
	// to services in the intranet.
	LoadBalancerClass = "wga.kraudcloud.com/intranet"
)

var lbcPredicate = &predicate.TypedFuncs[client.Object]{
	UpdateFunc: func(e event.UpdateEvent) bool {
		s, ok := e.ObjectNew.(*corev1.Service)
		if !ok {
			return false
		}

		return isLoadBalancerClass(s) && hasNoIP(s)
	},
	CreateFunc: func(e event.CreateEvent) bool {
		s, ok := e.Object.(*corev1.Service)
		if !ok {
			return false
		}

		return isLoadBalancerClass(s) && hasNoIP(s)
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		s, ok := e.Object.(*corev1.Service)
		if !ok {
			return false
		}

		return isLoadBalancerClass(s) && hasNoIP(s)
	},
	GenericFunc: func(e event.GenericEvent) bool {
		s, ok := e.Object.(*corev1.Service)
		if !ok {
			return false
		}

		return isLoadBalancerClass(s) && hasNoIP(s)
	},
}

func isLoadBalancerClass(service *corev1.Service) bool {
	return service != nil &&
		service.Spec.LoadBalancerClass != nil &&
		*service.Spec.LoadBalancerClass == LoadBalancerClass &&
		service.Spec.Type == corev1.ServiceTypeLoadBalancer
}

func hasNoIP(service *corev1.Service) bool {
	return service == nil || len(service.Status.LoadBalancer.Ingress) == 0 ||
		service.Status.LoadBalancer.Ingress[0].IP == ""

}

func registerLoadBalancerReconciler(mgr ctrl.Manager, serviceNets []net.IPNet, log *slog.Logger) {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WithEventFilter(lbcPredicate).
		Owns(&corev1.Service{}, builder.WithPredicates(lbcPredicate)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(o)}}
		}), builder.WithPredicates(lbcPredicate)).
		Complete(reconcile.AsReconciler(mgr.GetClient(), &LoadBalancerClassReconciler{
			client:      mgr.GetClient(),
			serviceNets: serviceNets,
			log:         log.With("component", "service-controller"),
		}))
	if err != nil {
		slog.Error("unable to create service-controller", "err", err)
		os.Exit(1)
	}
}

type LoadBalancerClassReconciler struct {
	client      client.Client
	serviceNets []net.IPNet
	log         *slog.Logger
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Compare the state specified by the LokiStack object against the actual cluster state,
// and then perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *LoadBalancerClassReconciler) Reconcile(ctx context.Context, svc *corev1.Service) (reconcile.Result, error) {
	// We know we need to assign an ip.
	if len(svc.Status.LoadBalancer.Ingress) != 0 {
		return reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("service already has an ip"))
	}

	r.log.Info("assigning ip to service", "service", svc.Name)

	ip := net.ParseIP(svc.Spec.LoadBalancerIP)
	var err error
	// if the user didn't specify an ip, allocate one
	if ip == nil {
		// pick a random net
		net := randPick(r.serviceNets)
		// allocate a rand ip in that net
		ip, err = cidr.HostBig(&net, generateIndex(&net))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to allocate ip: %w", err)
		}
	}

	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: ip.String()}}
	err = r.client.Status().Update(ctx, svc)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to update service status: %w", err)
	}

	return reconcile.Result{}, nil
}
