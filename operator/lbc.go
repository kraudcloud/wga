package operator

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/apparentlymart/go-cidr/cidr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// LoadBalancerIPs is a comma separated list of ips that the user wants to assign to the service.
	// It should respect the ip families of the service.
	LoadBalancerIPs = "wga.kraudcloud.com/loadBalancerIPs"
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
		return reconcile.Result{}, nil
	}

	r.log.Info("assigning ip to service", "service", svc.Name)

	var generated *net.IP
	serviceIPs := []net.IP{}
	if len(svc.Annotations[LoadBalancerIPs]) == 0 {
		ip, err := cidr.HostBig(&r.serviceNets[0], generateIndex(time.Now(), maskBits(r.serviceNets[0])))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to generate ip: %w", err)
		}

		generated = &ip
		serviceIPs = append(serviceIPs, ip)
	} else {
		ips := strings.Split(svc.Annotations[LoadBalancerIPs], ",")
		for _, ip := range ips {
			ip := net.ParseIP(strings.TrimSpace(ip))
			if ip == nil {
				return reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("invalid ip: %s", ip))
			}

			serviceIPs = append(serviceIPs, ip)
		}
	}

	// check for conflicts. Query all services in the cluster.
	services := &corev1.ServiceList{}
	err := r.client.List(ctx, services)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list services: %w", err)
	}

	for _, service := range services.Items {
		if service.UID == svc.UID {
			continue
		}

		for _, ingress := range service.Status.LoadBalancer.Ingress {
			for _, ip := range serviceIPs {
				if ingress.IP == ip.String() {
					// retry if the one we generated conflicts
					if generated != nil && ip.String() == generated.String() {
						return reconcile.Result{}, fmt.Errorf("generated ip %s is already assigned to another service", ip.String())
					}

					if len(svc.Status.Conditions) > 0 && svc.Status.Conditions[len(svc.Status.Conditions)-1].Reason == "InvalidLoadBalancerIP" {
						return reconcile.Result{}, fmt.Errorf("generated ip %s is already assigned to another service", ip.String())
					}

					svc.Status.Conditions = append(svc.Status.Conditions, metav1.Condition{
						Type:               "Failed",
						Status:             metav1.ConditionFalse,
						ObservedGeneration: svc.Generation,
						LastTransitionTime: metav1.Now(),
						Reason:             "InvalidLoadBalancerIP",
						Message:            fmt.Sprintf("The requested ip (%s) is already assigned to another service", ip.String()),
					})
					err := r.client.Status().Update(ctx, svc)
					if err != nil {
						return reconcile.Result{}, fmt.Errorf("unable to update service status: %w", err)
					}

					return reconcile.Result{}, nil
				}
			}
		}
	}

	// now that we figured all ips we have are unique, let's assign them.
	ports := []corev1.PortStatus{}
	for _, port := range svc.Spec.Ports {
		ports = append(ports, corev1.PortStatus{
			Protocol: port.Protocol,
			Port:     port.Port,
		})
	}

	for _, ip := range serviceIPs {
		svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{
			IP:    ip.String(),
			Ports: ports,
		})
	}

	// set additional status fields
	svc.Status.Conditions = append(svc.Status.Conditions, metav1.Condition{
		Type:               "Active",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: svc.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "LoadBalancerReady",
		Message:            "LoadBalancer is ready",
	})

	err = r.client.Status().Update(ctx, svc)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to update service status: %w", err)
	}

	return reconcile.Result{}, nil
}
