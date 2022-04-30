/*
Copyright 2022 DataPunch Project

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eks

import (
	"fmt"
	"github.com/datapunchorg/punch/pkg/framework"
	"github.com/datapunchorg/punch/pkg/resource"
)

const (
	ToBeReplacedS3BucketName           = "todo_use_your_own_bucket_name"
	DefaultInstanceType                = "t3.large"
	DefaultNodeGroupSize               = 3
	DefaultMaxNodeGroupSize            = 10
	DefaultNginxIngressHelmInstallName = "ingress-nginx"
	DefaultNginxIngressNamespace       = "ingress-nginx"
	DefaultNginxEnableHttp             = true
	DefaultNginxEnableHttps            = true

	KindEksTopology = "Eks"

	CmdEnvHelmExecutable             = "helmExecutable"
	CmdEnvWithMinikube               = "withMinikube"
	CmdEnvNginxHelmChart             = "nginxHelmChart"
	CmdEnvClusterAutoscalerHelmChart = "ClusterAutoscalerHelmChart"
	CmdEnvKubeConfig                 = "kubeConfig"

	DefaultVersion        = "datapunch.org/v1alpha1"
	DefaultRegion         = "us-west-1"
	DefaultNamePrefix     = "my"
	DefaultHelmExecutable = "helm"
)

type EksTopology struct {
	framework.TopologyBase               `json:",inline" yaml:",inline"`
	Spec       EksTopologySpec            `json:"spec"`
}

type EksTopologySpec struct {
	NamePrefix        string                   `json:"namePrefix" yaml:"namePrefix"`
	Region            string                   `json:"region"`
	VpcId             string                   `json:"vpcId" yaml:"vpcId"`
	S3BucketName      string                   `json:"s3BucketName" yaml:"s3BucketName"`
	S3Policy          resource.IAMPolicy       `json:"s3Policy" yaml:"s3Policy"`
	Eks               resource.EKSCluster  `json:"eks" yaml:"eks"`
	NodeGroups        []resource.NodeGroup `json:"nodeGroups" yaml:"nodeGroups"`
	NginxIngress      NginxIngress             `json:"nginxIngress" yaml:"nginxIngress"`
	AutoScaling       resource.AutoScalingSpec `json:"autoScaling" yaml:"autoScaling"`
}

type NginxIngress struct {
	HelmInstallName string `json:"helmInstallName" yaml:"helmInstallName"`
	Namespace       string `json:"namespace" yaml:"namespace"`
	EnableHttp      bool   `json:"enableHttp" yaml:"enableHttp"`
	EnableHttps     bool   `json:"enableHttps" yaml:"enableHttps"`
}

func GenerateEksTopology() EksTopology {
	namePrefix := "{{ or .Values.namePrefix `my` }}"
	s3BucketName := "{{ or .Values.s3BucketName .DefaultS3BucketName }}"

	topology := CreateDefaultEksTopology(namePrefix, s3BucketName)

	topology.Spec.Region = "{{ or .Values.region `us-west-1` }}"
	topology.Spec.VpcId = "{{ or .Values.vpcId .DefaultVpcId }}"

	topology.Metadata.CommandEnvironment[CmdEnvHelmExecutable] = "helm"
	topology.Metadata.CommandEnvironment[CmdEnvWithMinikube] = "false"
	topology.Metadata.CommandEnvironment[CmdEnvNginxHelmChart] = "helm-charts/ingress-nginx/charts/ingress-nginx"
	topology.Metadata.CommandEnvironment[CmdEnvClusterAutoscalerHelmChart] = "helm-charts/cluster-autoscaler/charts/cluster-autoscaler"
	topology.Metadata.CommandEnvironment[CmdEnvKubeConfig] = ""

	topology.Metadata.Notes["apiUserPassword"] = "Please make sure to provide API gateway user password when deploying the topology, e.g. --set apiUserPassword=your-password"

	return topology
}

func CreateDefaultEksTopology(namePrefix string, s3BucketName string) EksTopology {
	topologyName := fmt.Sprintf("%s-Eks-k8s", namePrefix)
	k8sClusterName := fmt.Sprintf("%s-k8s-01", namePrefix)
	controlPlaneRoleName := fmt.Sprintf("%s-eks-control-plane", namePrefix)
	instanceRoleName := fmt.Sprintf("%s-eks-instance", namePrefix)
	securityGroupName := fmt.Sprintf("%s-eks-sg-01", namePrefix)
	nodeGroupName := fmt.Sprintf("%s-ng-01", k8sClusterName)
	topology := EksTopology{
		TopologyBase: framework.TopologyBase{
			ApiVersion: DefaultVersion,
			Kind:       KindEksTopology,
			Metadata: framework.TopologyMetadata{
				Name: topologyName,
				CommandEnvironment: map[string]string{
					CmdEnvHelmExecutable: DefaultHelmExecutable,
				},
				Notes: map[string]string{},
			},
		},
		Spec: EksTopologySpec{
			NamePrefix:   namePrefix,
			Region:       DefaultRegion,
			VpcId:        "",
			S3BucketName: s3BucketName,
			S3Policy:     resource.IAMPolicy{},
			Eks: resource.EKSCluster{
				ClusterName: k8sClusterName,
				// TODO fill in default value for SubnetIds
				ControlPlaneRole: resource.IAMRole{
					Name:                     controlPlaneRoleName,
					AssumeRolePolicyDocument: framework.DefaultEKSAssumeRolePolicyDocument,
					ExtraPolicyArns: []string{
						"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
					},
				},
				InstanceRole: resource.IAMRole{
					Name:                     instanceRoleName,
					AssumeRolePolicyDocument: framework.DefaultEC2AssumeRolePolicyDocument,
					ExtraPolicyArns: []string{
						"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
						"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
						"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
						"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
					},
				},
				SecurityGroups: []resource.SecurityGroup{
					{
						Name: securityGroupName,
						InboundRules: []resource.SecurityGroupInboundRule{
							{
								IPProtocol: "-1",
								FromPort:   -1,
								ToPort:     -1,
								IPRanges:   []string{"0.0.0.0/0"},
							},
						},
					},
				},
			},
			AutoScaling: resource.AutoScalingSpec{
				EnableClusterAutoscaler: false,
				ClusterAutoscalerIAMRole: resource.IAMRole{
					Name: fmt.Sprintf("%s-cluster-autoscaler-role", namePrefix),
					Policies: []resource.IAMPolicy{
						resource.IAMPolicy{
							Name: fmt.Sprintf("%s-cluster-autoscaler-policy", namePrefix),
							PolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
                "autoscaling:DescribeAutoScalingGroups",
                "autoscaling:DescribeAutoScalingInstances",
                "autoscaling:DescribeLaunchConfigurations",
                "autoscaling:DescribeTags",
                "autoscaling:SetDesiredCapacity",
                "autoscaling:TerminateInstanceInAutoScalingGroup",
                "ec2:DescribeLaunchTemplateVersions",
                "ec2:DescribeInstanceTypes"
            ],
            "Resource": "*",
            "Effect": "Allow"
        }
    ]
}`,
						},
					},
				},
			},
			NodeGroups: []resource.NodeGroup{
				{
					Name:          nodeGroupName,
					InstanceTypes: []string{DefaultInstanceType},
					DesiredSize:   DefaultNodeGroupSize,
					MaxSize:       DefaultMaxNodeGroupSize,
					MinSize:       DefaultNodeGroupSize,
				},
			},
			NginxIngress: NginxIngress{
				HelmInstallName: DefaultNginxIngressHelmInstallName,
				Namespace:       DefaultNginxIngressNamespace,
				EnableHttp:      DefaultNginxEnableHttp,
				EnableHttps:     DefaultNginxEnableHttps,
			},
		},
	}
	UpdateEksTopologyByS3BucketName(&topology.Spec, s3BucketName)
	return topology
}

func UpdateEksTopologyByS3BucketName(spec *EksTopologySpec, s3BucketName string) {
	spec.S3BucketName = s3BucketName
	spec.S3Policy.Name = fmt.Sprintf("%s-s3", s3BucketName)
	spec.S3Policy.PolicyDocument = fmt.Sprintf(`{"Version":"2012-10-17","Statement":[
{"Effect":"Allow","Action":"s3:*","Resource":["arn:aws:s3:::%s", "arn:aws:s3:::%s/*"]}
]}`, s3BucketName, s3BucketName)
}

func (t *EksTopology) GetKind() string {
	return t.Kind
}

func (t *EksTopology) GetMetadata() *framework.TopologyMetadata {
	return &t.Metadata
}
