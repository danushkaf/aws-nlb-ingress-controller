package cloudformation

import (
	"fmt"

	cfn "github.com/awslabs/goformation/v4/cloudformation"
	"github.com/awslabs/goformation/v4/cloudformation/ec2"
	"github.com/awslabs/goformation/v4/cloudformation/elasticloadbalancingv2"
	"github.com/awslabs/goformation/v4/cloudformation/tags"
	"github.com/danushkaf/aws-nlb-ingress-controller/pkg/network"
)

//const is constance values for resource naming used to build cf templates
const (
	AWSStackName                     = "AWS::StackName"
	AWSRegion                        = "AWS::Region"
	LoadBalancerResourceName         = "LoadBalancer"
	ListnerResourceName              = "Listener"
	SecurityGroupIngressResourceName = "SecurityGroupIngress"
	TargetGroupResourceName          = "TargetGroup"
	OutputKeyNLBEndpoint             = "NLBHostName"
)

func buildAWSElasticLoadBalancingV2Listener() *elasticloadbalancingv2.Listener {
	return &elasticloadbalancingv2.Listener{
		LoadBalancerArn: cfn.Ref(LoadBalancerResourceName),
		Protocol:        "TCP",
		Port:            80,
		DefaultActions: []elasticloadbalancingv2.Listener_Action{
			elasticloadbalancingv2.Listener_Action{
				TargetGroupArn: cfn.Ref(TargetGroupResourceName),
				Type:           "forward",
			},
		},
	}
}

func buildAWSElasticLoadBalancingV2LoadBalancer(subnetIDs []string) *elasticloadbalancingv2.LoadBalancer {
	return &elasticloadbalancingv2.LoadBalancer{
		IpAddressType: "ipv4",
		Scheme:        "internal",
		Subnets:       subnetIDs,
		Tags: []tags.Tag{
			{
				Key:   "com.github.amazon-nlb-ingress-controller/stack",
				Value: cfn.Ref(AWSStackName),
			},
		},
		Type: "network",
	}
}

func buildAWSElasticLoadBalancingV2TargetGroup(vpcID string, instanceIDs []string, nodePort int, dependsOn []string) *elasticloadbalancingv2.TargetGroup {
	targets := make([]elasticloadbalancingv2.TargetGroup_TargetDescription, len(instanceIDs))
	for i, instanceID := range instanceIDs {
		targets[i] = elasticloadbalancingv2.TargetGroup_TargetDescription{Id: instanceID}
	}

	return &elasticloadbalancingv2.TargetGroup{
		HealthCheckIntervalSeconds: 30,
		HealthCheckPort:            "traffic-port",
		HealthCheckProtocol:        "TCP",
		HealthCheckTimeoutSeconds:  10,
		HealthyThresholdCount:      3,
		Port:                       nodePort,
		Protocol:                   "TCP",
		Tags: []tags.Tag{
			{
				Key:   "com.github.amazon-nlb-ingress-controller/stack",
				Value: cfn.Ref(AWSStackName),
			},
		},
		TargetType:              "instance",
		Targets:                 targets,
		UnhealthyThresholdCount: 3,
		VpcId:                   vpcID,
	}
}

func buildAWSEC2SecurityGroupIngresses(securityGroupIds []string, cidr string, nodePort int) []*ec2.SecurityGroupIngress {
	sgIngresses := make([]*ec2.SecurityGroupIngress, len(securityGroupIds))
	for i, sgID := range securityGroupIds {
		sgIngresses[i] = &ec2.SecurityGroupIngress{
			IpProtocol: "TCP",
			CidrIp:     cidr,
			FromPort:   nodePort,
			ToPort:     nodePort,
			GroupId:    sgID,
		}
	}

	return sgIngresses
}

//TemplateConfig is the structure of configuration used to provide data to build the cf template
type TemplateConfig struct {
	Network  *network.Network
	NodePort int
}

// BuildNLBTemplateFromIngressRule generates the cloudformation template according to the config provided
func BuildNLBTemplateFromIngressRule(cfg *TemplateConfig) *cfn.Template {
	template := cfn.NewTemplate()

	targetGroup := buildAWSElasticLoadBalancingV2TargetGroup(*cfg.Network.Vpc.VpcId, cfg.Network.InstanceIDs, cfg.NodePort, []string{LoadBalancerResourceName})
	template.Resources[TargetGroupResourceName] = targetGroup

	listener := buildAWSElasticLoadBalancingV2Listener()
	template.Resources[ListnerResourceName] = listener

	securityGroupIngresses := buildAWSEC2SecurityGroupIngresses(cfg.Network.SecurityGroupIDs, *cfg.Network.Vpc.CidrBlock, cfg.NodePort)
	for i, sgI := range securityGroupIngresses {
		template.Resources[fmt.Sprintf("%s%d", SecurityGroupIngressResourceName, i)] = sgI
	}

	loadBalancer := buildAWSElasticLoadBalancingV2LoadBalancer(cfg.Network.SubnetIDs)
	template.Resources[LoadBalancerResourceName] = loadBalancer

	template.Outputs = map[string]interface{}{
		OutputKeyNLBEndpoint: Output{Value: cfn.GetAtt(LoadBalancerResourceName, "DNSName")},
	}

	return template
}
