/*
Copyright 2022 DataPunch Organization

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

package main

import (
	"fmt"
	"strings"

	"github.com/datapunchorg/punch/pkg/framework"
)

type TopologyHandler struct {
}

var Handler TopologyHandler

func (t *TopologyHandler) Generate() (framework.Topology, error) {
	namePrefix := framework.DefaultNamePrefixTemplate
	topology := CreateDefaultRdsDatabaseTopology(namePrefix)
	return &topology, nil
}

func (t *TopologyHandler) Validate(topology framework.Topology, phase string) (framework.Topology, error) {
	resolvedDatabaseTopology := topology.(*RdsDatabaseTopology)

	if strings.EqualFold(phase, framework.PhaseBeforeInstall) {
		if resolvedDatabaseTopology.Spec.MasterUserPassword == "" || resolvedDatabaseTopology.Spec.MasterUserPassword == framework.TemplateNoValue {
			return nil, fmt.Errorf("spec.masterUserPassword is emmpty, please provide the value for the password")
		}
	}

	return topology, nil
}

func (t *TopologyHandler) Install(topology framework.Topology) (framework.DeploymentOutput, error) {
	databaseTopology := topology.(*RdsDatabaseTopology)
	deployment := framework.NewDeployment()
	deployment.AddStep("createDatabase", "Create database", func(c framework.DeploymentContext) (framework.DeployableOutput, error) {
		result, err := CreateDatabase(databaseTopology.Spec)
		if err != nil {
			return framework.NewDeploymentStepOutput(), err
		}
		return framework.DeployableOutput{"endpoint": *result.Endpoint}, nil
	})
	err := deployment.Run()
	return deployment.GetOutput(), err
}

func (t *TopologyHandler) Uninstall(topology framework.Topology) (framework.DeploymentOutput, error) {
	databaseTopology := topology.(*RdsDatabaseTopology)
	deployment := framework.NewDeployment()
	deployment.AddStep("deleteDatabase", "Delete database", func(c framework.DeploymentContext) (framework.DeployableOutput, error) {
		region := databaseTopology.Spec.Region
		databaseId := databaseTopology.Spec.DatabaseId
		err := DeleteDatabase(region, databaseId)
		return framework.NewDeploymentStepOutput(), err
	})
	err := deployment.Run()
	return deployment.GetOutput(), err
}

// The name for your database of up to 64 alphanumeric characters.
// TODO check length
func normalizeDatabaseName(name string) string {
	return nonAlphanumericRegexp.ReplaceAllString(name, "")
}
