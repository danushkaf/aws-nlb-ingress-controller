package ingress

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	cfn "github.com/danushkaf/aws-nlb-ingress-controller/pkg/cloudformation"
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

func shouldUpdate(stack *cloudformation.Stack, instance *extensionsv1beta1.Ingress, r *ReconcileIngress) bool {
	rulePaths, err := json.Marshal(instance.Spec.Rules[0].HTTP.Paths)
	var rulePathsStr string
	if err != nil {
		rulePathsStr = ""
	} else {
		rulePathsStr = string(rulePaths)
	}
	if rulePathsStr != cfn.StackOutputMap(stack)[cfn.OutputKeyIngressRules] {
		r.log.Info("Rules in Outputs are not matching, Should Update")
		return true
	} else {
		r.log.Debug("Rules in Outputs are matching, Should Update not triggered.")
	}
	return false
}
