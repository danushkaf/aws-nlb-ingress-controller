package ingress

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/danushkaf/aws-nlb-ingress-controller/pkg/finalizers"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type mockCloudformation struct {
	cloudformationiface.CloudFormationAPI
	Stacks map[string]*cloudformation.Stack
}

func (m *mockCloudformation) CreateStack(in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	if *in.StackName == "brokenCreate" {
		return nil, fmt.Errorf("mockCloudformation.CreateStack failed")
	}

	return &cloudformation.CreateStackOutput{}, nil
}

func (m *mockCloudformation) UpdateStack(in *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
	if *in.StackName == "brokenStackUpdate" {
		return nil, fmt.Errorf("mockCloudformation.UpdateStack failed")
	}
	return &cloudformation.UpdateStackOutput{}, nil
}

func (m *mockCloudformation) DescribeStacks(in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	if *in.StackName == "broken" {
		return nil, fmt.Errorf("mockCloudformation.DescribeStacks failed")
	}

	if s, ok := m.Stacks[*in.StackName]; ok {
		return &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{s},
		}, nil
	}

	return nil, awserr.New("ValidationError", fmt.Sprintf("Stack with id %s does not exist", *in.StackName), fmt.Errorf(""))
}

func (m *mockCloudformation) DeleteStack(in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	if *in.StackName == "brokenDelete" {
		return nil, fmt.Errorf("mockCloudformation.DeleteStack failed")
	}

	if _, ok := m.Stacks[*in.StackName]; ok {
		delete(m.Stacks, *in.StackName)
		return &cloudformation.DeleteStackOutput{}, nil
	}

	return nil, awserr.New("ValidationError", fmt.Sprintf("Stack with id %s does not exist", *in.StackName), fmt.Errorf(""))
}

func (m *mockCloudformation) ListStackResources(in *cloudformation.ListStackResourcesInput) (*cloudformation.ListStackResourcesOutput, error) {

	if _, ok := m.Stacks[*in.StackName]; ok {
		return &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []*cloudformation.StackResourceSummary{
				{
					LogicalResourceId:  aws.String("TargetGroup"),
					PhysicalResourceId: aws.String("tgroupARN"),
				},
			},
		}, nil
	}

	return nil, awserr.New("ValidationError", fmt.Sprintf("Cannot get targetgroup in %s stack", *in.StackName), fmt.Errorf(""))
}

type mockEC2 struct {
	ec2iface.EC2API
	getASGTag bool
}

func (m *mockEC2) DescribeVpcs(in *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{
		Vpcs: []*ec2.Vpc{
			&ec2.Vpc{
				VpcId:     aws.String("vpc-foobar"),
				CidrBlock: aws.String("10.0.0.0/32"),
			},
		},
	}, nil
}

func (m *mockEC2) DescribeInstances(in *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if m.getASGTag {
		return &ec2.DescribeInstancesOutput{
			Reservations: []*ec2.Reservation{
				&ec2.Reservation{
					Instances: []*ec2.Instance{
						&ec2.Instance{
							VpcId:    aws.String("vpc-foobar"),
							SubnetId: aws.String("sub-foobar"),
							SecurityGroups: []*ec2.GroupIdentifier{
								&ec2.GroupIdentifier{
									GroupId: aws.String("sg-foobar"),
								},
							},
							Tags: []*ec2.Tag{
								{
									Key:   aws.String("aws:autoscaling:groupName"),
									Value: aws.String("asg-foobar"),
								},
							},
						},
					},
				},
			},
		}, nil
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			&ec2.Reservation{
				Instances: []*ec2.Instance{
					&ec2.Instance{
						VpcId:    aws.String("vpc-foobar"),
						SubnetId: aws.String("sub-foobar"),
						SecurityGroups: []*ec2.GroupIdentifier{
							&ec2.GroupIdentifier{
								GroupId: aws.String("sg-foobar"),
							},
						},
					},
				},
			},
		},
	}, nil
}

type mockAutoscaling struct {
	autoscalingiface.AutoScalingAPI
	withTargetGroupARN bool
	describeErr        bool
	attachTGErr        bool
	detachTGErr        bool
}

func (m *mockAutoscaling) DescribeAutoScalingGroups(in *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if m.withTargetGroupARN {
		return &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					VPCZoneIdentifier: aws.String("sub-foobar,sub-extra"),
					TargetGroupARNs:   aws.StringSlice([]string{"tgroupARN"}),
				},
			},
		}, nil
	}

	if m.describeErr {
		return nil, awserr.New("ValidationError", "cannot describe ASG", fmt.Errorf(""))
	}

	return &autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			{
				VPCZoneIdentifier: aws.String("sub-foobar"),
			},
		},
	}, nil
}

func (m *mockAutoscaling) AttachLoadBalancerTargetGroups(in *autoscaling.AttachLoadBalancerTargetGroupsInput) (*autoscaling.AttachLoadBalancerTargetGroupsOutput, error) {
	if m.attachTGErr {
		return nil, awserr.New("ValidationError", "attach error", fmt.Errorf(""))
	}
	return &autoscaling.AttachLoadBalancerTargetGroupsOutput{}, nil
}

func (m *mockAutoscaling) DetachLoadBalancerTargetGroups(in *autoscaling.DetachLoadBalancerTargetGroupsInput) (*autoscaling.DetachLoadBalancerTargetGroupsOutput, error) {
	if m.detachTGErr {
		return nil, awserr.New("ValidationError", "attach error", fmt.Errorf(""))
	}
	return &autoscaling.DetachLoadBalancerTargetGroupsOutput{}, nil
}

func newMockIngress(name string, isDeleted, hasFinalizer bool) *extensionsv1beta1.Ingress {
	instance := &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Annotations: map[string]string{
				IngressClassAnnotation:      "nlb",
				IngressAnnotationNginxImage: "nginx:latest",
			},
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				extensionsv1beta1.IngressRule{
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								extensionsv1beta1.HTTPIngressPath{
									Path: "/api/v1/foobar",
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: "foo",
										ServicePort: intstr.FromInt(30123),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if isDeleted {
		instance.ObjectMeta.DeletionTimestamp = func() *metav1.Time {
			t1, _ := time.Parse(time.RFC3339, "2012-11-01T22:08:41+00:00")
			t := metav1.NewTime(t1)
			return &t
		}()
	}

	if hasFinalizer {
		instance.SetFinalizers(finalizers.AddFinalizer(instance, FinalizerCFNStack))
	}

	return instance

}

func newMockService(name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: createReverseProxyResourceName(name), Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{NodePort: 30123}}},
	}
}

func newMockNodeList() *corev1.NodeList {
	return &corev1.NodeList{
		Items: []corev1.Node{
			corev1.Node{
				Spec: corev1.NodeSpec{
					ProviderID: "aws:///us-west-2b/i-07d8783206d39591d",
				},
			},
		},
	}
}
