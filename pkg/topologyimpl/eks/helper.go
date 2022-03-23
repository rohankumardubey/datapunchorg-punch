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
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/datapunchorg/punch/pkg/awslib"
	"github.com/datapunchorg/punch/pkg/common"
	"github.com/datapunchorg/punch/pkg/framework"
	"github.com/datapunchorg/punch/pkg/kubelib"
	"github.com/datapunchorg/punch/pkg/resource"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"strings"
	"time"
)

func CreateInstanceIAMRole(topology EksTopology) string {
	region := topology.Spec.Region
	roleName, err := resource.CreateIAMRoleWithMorePolicies(region, topology.Spec.EKS.InstanceRole, []resource.IAMPolicy{topology.Spec.S3Policy})
	if err != nil {
		// TODO remove Fatalf
		log.Fatalf("Failed to create instance IAM role: %s", err.Error())
	}
	return roleName
}

// CreateClusterAutoscalerIAMRole returns IAM Role name like: AmazonEKSClusterAutoscalerRole-xxx
func CreateClusterAutoscalerIAMRole(topology EksTopology, oidcId string) (string, error) {
	region := topology.Spec.Region
	session := awslib.CreateSession(region)
	accountId, err := awslib.GetCurrentAccount(session)
	if err != nil {
		return "", fmt.Errorf("failed to get current account: %s", err.Error())
	}
	role := topology.Spec.AutoScaling.ClusterAutoscalerIAMRole
	role.AssumeRolePolicyDocument = fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "arn:aws:iam::%s:oidc-provider/oidc.eks.us-west-1.amazonaws.com/id/%s"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "oidc.eks.us-west-1.amazonaws.com/id/%s:aud": "sts.amazonaws.com",
                    "oidc.eks.us-west-1.amazonaws.com/id/%s:sub": "system:serviceaccount:kube-system:cluster-autoscaler"
                }
            }
        }
    ]
}
`, accountId, oidcId, oidcId, oidcId)
	role.Name += "-" + oidcId
	err = resource.CreateIAMRole(region, role)
	if err != nil {
		return "", fmt.Errorf("failed to create cluster autoscaler IAM role: %s", err.Error())
	}
	return role.Name, nil
}

func CreateClusterAutoscalerIAMServiceAccount(commandEnvironment framework.CommandEnvironment, topology EksTopology, iamRoleName string) error {
	region := topology.Spec.Region
	session := awslib.CreateSession(region)
	accountId, err := awslib.GetCurrentAccount(session)
	if err != nil {
		return fmt.Errorf("failed to get current account: %s", err.Error())
	}
	roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", accountId, iamRoleName)
	systemNamespace := "kube-system"
	serviceAccountName := "cluster-autoscaler"
	awslib.RunEksCtlCmd("eksctl",
		[]string{"create", "iamserviceaccount",
			"--name", serviceAccountName,
			"--region", topology.Spec.Region,
			"--cluster", topology.Spec.EKS.ClusterName,
			"--namespace", systemNamespace,
			"--attach-role-arn", roleArn,
			"--approve"})
	err = WaitEKSServiceAccount(commandEnvironment, topology, systemNamespace, serviceAccountName)
	return err
}

func WaitEKSServiceAccount(commandEnvironment framework.CommandEnvironment, topology EksTopology, namespace string, serviceAccount string) error {
	region := topology.Spec.Region
	clusterName := topology.Spec.EKS.ClusterName

	_, clientset, err := awslib.CreateKubernetesClient(region, commandEnvironment.Get(CmdEnvKubeConfig), clusterName)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %s", err.Error())
	}
	err = common.RetryUntilTrue(func() (bool, error) {
		_, err = clientset.CoreV1().ServiceAccounts(namespace).Get(
			context.TODO(),
			serviceAccount,
			v12.GetOptions{},
		)
		if err != nil {
			fmt.Sprintf("Failed to get service account %s in namespace %s in EKS cluster %s", serviceAccount, namespace, clusterName)
			return false, nil
		}
		return true, nil
	},
		5*time.Minute,
		10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to wait service account %s in namespace %s in EKS cluster %s", serviceAccount, namespace, clusterName)
	}
	return nil
}

// TODO remove log.Fatalf
func DeployNginxIngressController(commandEnvironment framework.CommandEnvironment, topology EksTopology) map[string]interface{} {
	nginxNamespace := topology.Spec.NginxIngress.Namespace
	helmInstallName := topology.Spec.NginxIngress.HelmInstallName
	serviceName := "ingress-nginx-controller"
	region := topology.Spec.Region
	eksClusterName := topology.Spec.EKS.ClusterName
	kubeConfig, err := awslib.CreateKubeConfig(region, commandEnvironment.Get(CmdEnvKubeConfig), eksClusterName)
	if err != nil {
		log.Fatalf("Failed to get kube config: %s", err)
	}

	defer kubeConfig.Cleanup()

	arguments := []string{
		"--set", fmt.Sprintf("service.enableHttp=%t", topology.Spec.NginxIngress.EnableHttp),
		"--set", fmt.Sprintf("service.enableHttps=%t", topology.Spec.NginxIngress.EnableHttps),
	}

	kubelib.InstallHelm(commandEnvironment.Get(CmdEnvHelmExecutable), commandEnvironment.Get(CmdEnvNginxHelmChart), kubeConfig, arguments, helmInstallName, nginxNamespace)

	_, clientset, err := awslib.CreateKubernetesClient(region, commandEnvironment.Get(CmdEnvKubeConfig), eksClusterName)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %s", err.Error())
	}
	err = kubelib.WaitPodsInPhase(clientset, nginxNamespace, serviceName, v1.PodRunning)
	if err != nil {
		log.Fatalf("Pod %s*** in namespace %s is not in phase %s", serviceName, nginxNamespace, v1.PodRunning)
	}

	urls, err := kubelib.GetServiceLoadBalancerUrls(clientset, nginxNamespace, serviceName)
	if err != nil {
		log.Fatalf("Failed to get load balancers for service %s in namespace %s in cluster %s: %v", serviceName, nginxNamespace, eksClusterName, err)
	}

	for _, url := range urls {
		session := awslib.CreateSession(region)
		elbClient := elb.New(session)
		common.RetryUntilTrue(func() (bool, error) {
			instanceStates, err := awslib.GetLoadBalancerInstanceStatesByDNSName(elbClient, url)
			if err != nil {
				return false, err
			}
			if len(instanceStates) == 0 {
				log.Printf("Did not find instances for load balancer %s, wait and retry", url)
				return false, nil
			}
			for _, entry := range instanceStates {
				if strings.EqualFold(*entry.State, "InService") {
					return true, nil
				}
			}
			log.Printf("No ready instance for load balancer %s, wait and retry", url)
			return false, nil
		},
			10*time.Minute,
			10*time.Second)
	}

	output := make(map[string]interface{})
	output["loadBalancerUrls"] = urls
	return output
}

func DeleteEKSCluster(region string, clusterName string) {
	err := awslib.DeleteEKSCluster(region, clusterName)
	if err != nil {
		fmt.Sprintf("Failed to delete EKS cluster: %s", err.Error())
	}
	err = awslib.CheckEksCtlCmd("eksctl")
	if err == nil {
		awslib.RunEksCtlCmd("eksctl",
			[]string{"delete", "iamserviceaccount",
				"--region", region,
				"--cluster", clusterName,
				"--namespace", "kube-system",
				"--name", "cluster-autoscaler",
				"--wait",
			})
		awslib.RunEksCtlCmd("eksctl",
			[]string{"delete", "cluster",
				"--region", region,
				"--name", clusterName,
			})
	}
}