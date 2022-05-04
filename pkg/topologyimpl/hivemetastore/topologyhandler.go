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

package hivemetastore

import (
	"fmt"
	"github.com/datapunchorg/punch/pkg/framework"
	"github.com/datapunchorg/punch/pkg/kubelib"
	"github.com/datapunchorg/punch/pkg/topologyimpl/eks"
	"gopkg.in/yaml.v3"
	"regexp"
	"strings"
)

var nonAlphanumericRegexp *regexp.Regexp

func init() {
	framework.DefaultTopologyHandlerManager.AddHandler(KindHiveMetastoreTopology, &TopologyHandler{})

	var err error
	nonAlphanumericRegexp, err = regexp.Compile("[^a-zA-Z]+")
	if err != nil {
		panic(err)
	}
}

type TopologyHandler struct {
}

func (t *TopologyHandler) Generate() (framework.Topology, error) {
	topology := GenerateHiveMetastoreTopology()
	return &topology, nil
}

func (t *TopologyHandler) Parse(yamlContent []byte) (framework.Topology, error) {
	result := CreateDefaultHiveMetastoreTopology(DefaultNamePrefix, eks.ToBeReplacedS3BucketName)
	err := yaml.Unmarshal(yamlContent, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML (%s): \n%s", err.Error(), string(yamlContent))
	}
	return &result, nil
}

func (t *TopologyHandler) Validate(topology framework.Topology, phase string) (framework.Topology, error) {
	currentTopology := topology.(*HiveMetastoreTopology)

	if strings.EqualFold(phase, framework.PhaseBeforeInstall) {
		if currentTopology.Spec.Database.ExternalDb {
			if currentTopology.Spec.Database.Password == "" || currentTopology.Spec.Database.Password == framework.TemplateNoValue {
				return nil, fmt.Errorf("spec.dbUserPassword is emmpty, please provide the value for the password")
			}
		}
	}

	return topology, nil
}

func (t *TopologyHandler) Install(topology framework.Topology) (framework.DeploymentOutput, error) {
	currentTopology := topology.(*HiveMetastoreTopology)
	commandEnvironment := framework.CreateCommandEnvironment(currentTopology.Metadata.CommandEnvironment)
	if commandEnvironment.GetBoolOrElse(CmdEnvWithMinikube, false) {
		commandEnvironment.Set(CmdEnvKubeConfig, kubelib.GetKubeConfigPath())
	}
	deployment, err := eks.CreateInstallDeployment(currentTopology.Spec.EksSpec, commandEnvironment)
	if err != nil {
		return nil, err
	}
	if !currentTopology.Spec.Database.ExternalDb {
		deployment.AddStep("createHiveMetastoreDatabase", "Create Hive Metastore database", func(c framework.DeploymentContext) (framework.DeployableOutput, error) {
			databaseInfo, err := CreatePostgresqlDatabase(commandEnvironment, currentTopology.Spec)
			if err != nil {
				return framework.NewDeploymentStepOutput(), err
			}
			return framework.DeployableOutput{"databaseInfo": databaseInfo}, nil
		})
	}
	deployment.AddStep("initHiveMetastoreDatabase", "Init Hive Metastore database", func(c framework.DeploymentContext) (framework.DeployableOutput, error) {
		var databaseInfo DatabaseInfo
		if !currentTopology.Spec.Database.ExternalDb {
			databaseInfo = c.GetStepOutput("createHiveMetastoreDatabase")["databaseInfo"].(DatabaseInfo)
		} else {
			databaseInfo = DatabaseInfo{
				ConnectionString: currentTopology.Spec.Database.ConnectionString,
				User:             currentTopology.Spec.Database.User,
				Password:         currentTopology.Spec.Database.Password,
			}
		}
		err := InitDatabase(commandEnvironment, currentTopology.Spec, databaseInfo)
		if err != nil {
			return framework.NewDeploymentStepOutput(), err
		}
		return framework.NewDeploymentStepOutput(), nil
	})
	deployment.AddStep("installHiveMetastoreServer", "Install Hive Metastore server", func(c framework.DeploymentContext) (framework.DeployableOutput, error) {
		spec := currentTopology.Spec
		var databaseInfo DatabaseInfo
		if !spec.Database.ExternalDb {
			databaseInfo = c.GetStepOutput("createHiveMetastoreDatabase")["databaseInfo"].(DatabaseInfo)
		} else {
			databaseInfo = DatabaseInfo{
				ConnectionString: spec.Database.ConnectionString,
				User:             spec.Database.User,
				Password:         spec.Database.Password,
			}
		}
		urls, err := InstallMetastoreServer(commandEnvironment, spec, databaseInfo)
		if err != nil {
			return framework.NewDeploymentStepOutput(), err
		}
		if len(urls) == 0 {
			return framework.NewDeploymentStepOutput(), fmt.Errorf("did not get any load balancer url for hive metastore")
		}
		metastoreWarehouseDir := spec.WarehouseDir
		if commandEnvironment.GetBoolOrElse(CmdEnvWithMinikube, false) {
			metastoreWarehouseDir = WAREHOUSE_DIR_LOCAL_FILE_TEMP_DIRECTORY
		}
		return framework.DeployableOutput{
			"metastoreInClusterUrl": fmt.Sprintf("thrift://hive-metastore.%s.svc.cluster.local:9083", spec.Namespace),
			"metastoreLoadBalancerUrls": urls,
			"metastoreWarehouseDir": metastoreWarehouseDir,
		}, nil
	})
	err = deployment.Run()
	return deployment.GetOutput(), err
}

func (t *TopologyHandler) Uninstall(topology framework.Topology) (framework.DeploymentOutput, error) {
	currentTopology := topology.(*HiveMetastoreTopology)
	commandEnvironment := framework.CreateCommandEnvironment(currentTopology.Metadata.CommandEnvironment)

	deployment, err := eks.CreateUninstallDeployment(currentTopology.Spec.EksSpec, commandEnvironment)
	if err != nil {
		return nil, err
	}

	err = deployment.Run()
	return deployment.GetOutput(), err
}
