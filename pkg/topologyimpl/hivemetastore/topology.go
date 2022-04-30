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
	"github.com/datapunchorg/punch/pkg/topologyimpl/eks"
	"gopkg.in/yaml.v3"
)

const (
	KindHiveMetastoreTopology = "HiveMetastore"

	FieldMaskValue = "***"

	DefaultVersion    = "datapunch.org/v1alpha1"
	DefaultNamePrefix = "my"

	CmdEnvHelmExecutable             = "helmExecutable"
	CmdEnvWithMinikube               = "withMinikube"
	CmdEnvKubeConfig                 = "kubeConfig"
	CmdEnvHiveMetastoreCreateDatabaseHelmChart = "hiveMetastoreCreateDatabaseHelmChart"
	CmdEnvHiveMetastoreInitHelmChart = "hiveMetastoreInitHelmChart"
	CmdEnvHiveMetastoreServerHelmChart = "hiveMetastoreServerHelmChart"

	DefaultHelmExecutable = "helm"
)

type HiveMetastoreTopology struct {
	framework.TopologyBase               `json:",inline" yaml:",inline"`
	Spec       HiveMetastoreTopologySpec      `json:"spec"`
}

type HiveMetastoreTopologySpec struct {
	EksSpec               eks.EksTopologySpec   `json:"eksSpec" yaml:"eksSpec"`
    Namespace        string `json:"namespace" yaml:"namespace"`
	ImageRepository  string `json:"imageRepository" yaml:"imageRepository"`
	ImageTag string `json:"imageTag" yaml:"imageTag"`
	Database  HiveMetastoreDatabaseSpec `json:"database" yaml:"database"`
	WarehouseDir string `json:"warehouseDir" yaml:"warehouseDir"`
}

type HiveMetastoreDatabaseSpec struct {
	UseExternalDb    bool   `json:"useExternalDb" yaml:"useExternalDb"`
	ConnectionString string `json:"connectionString" yaml:"connectionString"`
	UserName       string `json:"userName" yaml:"userName"`
	UserPassword string `json:"userPassword" yaml:"userPassword"`
}

func GenerateHiveMetastoreTopology() HiveMetastoreTopology {
	namePrefix := "{{ or .Values.namePrefix `my` }}"
	s3BucketName := "{{ or .Values.s3BucketName .DefaultS3BucketName }}"

	topology := CreateDefaultHiveMetastoreTopology(namePrefix, s3BucketName)

	eksTopology := eks.GenerateEksTopology()
	topology.Spec.EksSpec = eksTopology.Spec

	return topology
}

func CreateDefaultHiveMetastoreTopology(namePrefix string, s3BucketName string) HiveMetastoreTopology {
	topologyName := fmt.Sprintf("%s-db-01", namePrefix)

	eksTopology := eks.CreateDefaultEksTopology(namePrefix, s3BucketName)

	topology := HiveMetastoreTopology{
		TopologyBase: framework.TopologyBase{
			ApiVersion: DefaultVersion,
			Kind: KindHiveMetastoreTopology,
			Metadata: framework.TopologyMetadata{
				Name:               topologyName,
				CommandEnvironment: map[string]string{
					CmdEnvHelmExecutable: DefaultHelmExecutable,
				    CmdEnvWithMinikube: "{{ or .Env.withMinikube `false` }}",
					CmdEnvHiveMetastoreCreateDatabaseHelmChart: "{{ or .Env.hiveMetastoreCreateDatabaseHelmChart `helm-charts/hive-metastore/charts/hive-metastore-postgresql-create-db` }}",
					CmdEnvHiveMetastoreInitHelmChart: "{{ or .Env.hiveMetastoreInitHelmChart `helm-charts/hive-metastore/charts/hive-metastore-init-postgresql` }}",
					CmdEnvHiveMetastoreServerHelmChart: "{{ or .Env.hiveMetastoreServerHelmChart `helm-charts/hive-metastore/charts/hive-metastore-server` }}",
				},
				Notes:              map[string]string{},
			},
		},
		Spec: HiveMetastoreTopologySpec{
			EksSpec:                eksTopology.Spec,
			Namespace: "hive-01",
			ImageRepository: "ghcr.io/datapunchorg/helm-hive-metastore",
			ImageTag: "main-1650942144",
			Database: HiveMetastoreDatabaseSpec{
				UseExternalDb:    false,
				ConnectionString: "",
				UserName:       "",
				UserPassword:   "",
			},
			WarehouseDir: fmt.Sprintf("s3a://%s/punch/hive/warehouse", s3BucketName),
		},
	}

	for k, v := range eksTopology.Metadata.CommandEnvironment {
		if _, ok := topology.Metadata.CommandEnvironment[k]; !ok {
			topology.Metadata.CommandEnvironment[k] = v
		}
	}

	eks.UpdateEksTopologyByS3BucketName(&topology.Spec.EksSpec, s3BucketName)

	return topology
}

func (t *HiveMetastoreTopology) GetKind() string {
	return t.Kind
}

func (t *HiveMetastoreTopology) String() string {
	topologyBytes, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Sprintf("(Failed to serialize topology: %s)", err.Error())
	}
	var copy HiveMetastoreTopology
	err = yaml.Unmarshal(topologyBytes, &copy)
	if err != nil {
		return fmt.Sprintf("(Failed to deserialize topology in ToYamlString(): %s)", err.Error())
	}
	if copy.Spec.Database.UserPassword != "" {
		copy.Spec.Database.UserPassword = FieldMaskValue
	}
	topologyBytes, err = yaml.Marshal(copy)
	if err != nil {
		return fmt.Sprintf("(Failed to serialize topology in String(): %s)", err.Error())
	}
	return string(topologyBytes)
}
