package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TlsSecretNameMask = "%s-service-tls"

func EnsureServiceSpec() func(current client.Object, expected client.Object) error {
	return func(current client.Object, expected client.Object) error {
		expectedService, ok := expected.(*corev1.Service)
		if !ok {
			return fmt.Errorf("expected a Service but got a %T", expected)
		}

		currentService, ok := current.(*corev1.Service)
		if !ok {
			return fmt.Errorf("expected a Service but got a %T", current)
		}

		currentService.Spec.Selector = expectedService.Spec.Selector

		resultPorts := make([]corev1.ServicePort, 0, len(expectedService.Spec.Ports))
		for _, port := range expectedService.Spec.Ports {
			currentPort := getPortByName(currentService.Spec.Ports, port.Name)
			if currentPort == nil {
				// add
				resultPorts = append(resultPorts, port)
			} else {
				// merge
				currentPort.Protocol = port.Protocol
				currentPort.Port = port.Port
				currentPort.TargetPort = port.TargetPort
				resultPorts = append(resultPorts, *currentPort)
			}
		}
		currentService.Spec.Ports = resultPorts

		return nil
	}
}

func CreateService(namespace string, name string, ports []corev1.ServicePort, labels map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    ports,
		},
	}
}

func GetInternalUrl(ctx context.Context, cli client.Client, namespace, serviceName string) (string, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
	}

	err := cli.Get(ctx, types.NamespacedName{
		Name:      serviceName,
		Namespace: namespace,
	}, svc)

	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace), nil
}

func getPortByName(ports []corev1.ServicePort, name string) *corev1.ServicePort {
	for _, port := range ports {
		if port.Name == name {
			return &port
		}
	}
	return nil
}
