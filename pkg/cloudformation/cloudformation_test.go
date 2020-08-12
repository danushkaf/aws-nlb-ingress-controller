package cloudformation

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	cfn "github.com/awslabs/goformation/v4/cloudformation"
	"github.com/danushkaf/aws-nlb-ingress-controller/pkg/network"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func getIngressRulesJsonStr() string {
	Rules := extensionsv1beta1.IngressRule{
		IngressRuleValue: extensionsv1beta1.IngressRuleValue{
			HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
				Paths: []extensionsv1beta1.HTTPIngressPath{
					{
						Path: "/api/v1/foobar",
						Backend: extensionsv1beta1.IngressBackend{
							ServiceName: "foobar-service",
							ServicePort: intstr.FromInt(8080),
						},
					},
				},
			},
		},
	}
	rulePaths, err := json.Marshal(Rules.IngressRuleValue.HTTP.Paths)
	var rulePathsStr string
	if err != nil {
		fmt.Println(err)
		rulePathsStr = ""
	} else {
		rulePathsStr = string(rulePaths)
	}
	return rulePathsStr
}

func TestBuildApiGatewayTemplateFromIngressRule(t *testing.T) {
	tests := []struct {
		name string
		args *TemplateConfig
		want *cfn.Template
	}{
		{
			name: "generates template",
			args: &TemplateConfig{
				Rule: extensionsv1beta1.IngressRule{
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Path: "/api/v1/foobar",
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: "foobar-service",
										ServicePort: intstr.FromInt(8080),
									},
								},
							},
						},
					},
				},
				Network: &network.Network{
					Vpc: &ec2.Vpc{
						VpcId:     aws.String("foo"),
						CidrBlock: aws.String("10.0.0.0/24"),
					},
					InstanceIDs:      []string{"i-foo"},
					SubnetIDs:        []string{"sn-foo"},
					SecurityGroupIDs: []string{"sg-foo"},
				},
				NodePort: 30123,
			},
			want: &cfn.Template{
				Resources: cfn.Resources{
					"TargetGroup":           buildAWSElasticLoadBalancingV2TargetGroup("foo", []string{"i-foo"}, 30123, []string{"LoadBalancer"}),
					"Listener":              buildAWSElasticLoadBalancingV2Listener(),
					"SecurityGroupIngress0": buildAWSEC2SecurityGroupIngresses([]string{"sg-foo"}, "10.0.0.0/24", 30123)[0],
					"LoadBalancer":          buildAWSElasticLoadBalancingV2LoadBalancer([]string{"sn-foo"}),
				},
				Outputs: map[string]interface{}{
					"NLBHostName":  Output{Value: cfn.GetAtt("LoadBalancer", "DNSName")},
					"IngressRules": Output{Value: getIngressRulesJsonStr()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildNLBTemplateFromIngressRule(tt.args)
			for k, resource := range got.Resources {
				if !reflect.DeepEqual(resource, tt.want.Resources[k]) {
					t.Errorf("Got Resources.%s = %v, want %v", k, got.Resources, tt.want.Resources)
				}
			}
			for k, resource := range got.Outputs {
				if !reflect.DeepEqual(resource, tt.want.Outputs[k]) {
					t.Errorf("Got Outputs.%s = %v, want %v", k, got.Outputs, tt.want.Outputs)
				}
			}
		})
	}
}
