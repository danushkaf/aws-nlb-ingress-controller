package ingress

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/labels"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

func getNodeSelector(ingress *extensionsv1beta1.Ingress) labels.Selector {
	s, err := labels.Parse(ingress.ObjectMeta.Annotations[IngressAnnotationNodeSelector])
	if err != nil {
		return DefaultNodeSelector
	}

	return s
}

func getNginxImage(ingress *extensionsv1beta1.Ingress) string {
	image, ok := ingress.ObjectMeta.Annotations[IngressAnnotationNginxImage]
	if ok {
		return image
	}

	return DefaultNginxImage
}

func getNginxServicePort(ingress *extensionsv1beta1.Ingress) int {
	port := ingress.ObjectMeta.Annotations[IngressAnnotationNginxServicePort]
	p, err := strconv.Atoi(port)
	if err != nil {
		return DefaultNginxServicePort
	}

	return p
}

func getNginxReplicas(ingress *extensionsv1beta1.Ingress) int {
	replicas := ingress.ObjectMeta.Annotations[IngressAnnotationNginxReplicas]
	r, err := strconv.Atoi(replicas)
	if err != nil {
		return DefaultNginxReplicas
	}

	return r
}

func createReverseProxyResourceName(name string) string {
	return fmt.Sprintf("%s-reverse-proxy", name)
}
