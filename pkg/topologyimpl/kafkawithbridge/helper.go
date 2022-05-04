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

package kafkawithbridge

import (
	"fmt"
	"github.com/datapunchorg/punch/pkg/awslib"
	"github.com/datapunchorg/punch/pkg/framework"
	"github.com/datapunchorg/punch/pkg/kubelib"
	"github.com/datapunchorg/punch/pkg/topologyimpl/eks"
	v1 "k8s.io/api/core/v1"
	"log"
)

func DeployKafkaBridge(commandEnvironment framework.CommandEnvironment, spec KafkaWithBridgeTopologySpec) error {
	region := spec.EksSpec.Region
	clusterName := spec.EksSpec.Eks.ClusterName
	_, clientset, err := awslib.CreateKubernetesClient(region, commandEnvironment.Get(eks.CmdEnvKubeConfig), clusterName)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %s", err.Error())
	}

	err = InstallKafkaBridgeHelm(commandEnvironment, spec)
	if err != nil {
		return fmt.Errorf("failed to install Spark History Server helm chart: %s", err.Error())
	}

	namespace := spec.KafkaBridge.Namespace
	podNamePrefix := "strimzi-kafka-bridge"
	err = kubelib.WaitPodsInPhases(clientset, namespace, podNamePrefix, []v1.PodPhase{v1.PodRunning})
	if err != nil {
		return fmt.Errorf("pod %s*** in namespace %s is not in phase %s", podNamePrefix, namespace, v1.PodRunning)
	}

	return nil
}

func InstallKafkaBridgeHelm(commandEnvironment framework.CommandEnvironment, spec KafkaWithBridgeTopologySpec) error {
	kubeConfig, err := awslib.CreateKubeConfig(spec.EksSpec.Region, commandEnvironment.Get(eks.CmdEnvKubeConfig), spec.EksSpec.Eks.ClusterName)
	if err != nil {
		log.Fatalf("Failed to get kube config: %s", err)
	}

	defer kubeConfig.Cleanup()

	installName := spec.KafkaBridge.HelmInstallName
	installNamespace := spec.KafkaBridge.Namespace

	arguments := []string{
		"--set", fmt.Sprintf("image.name=%s", spec.KafkaBridge.Image),
		"--set", fmt.Sprintf("kafka.bootstrapServers=%s", spec.KafkaBridge.KafkaBootstrapServers),
	}

	kubelib.InstallHelm(commandEnvironment.Get(eks.CmdEnvHelmExecutable), commandEnvironment.Get(CmdEnvKafkaBridgeHelmChart), kubeConfig, arguments, installName, installNamespace)

	return nil
}