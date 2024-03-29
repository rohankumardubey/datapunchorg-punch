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

package framework

var DefaultTopologyHandlerManager = TopologyHandlerManager{
	handlers: map[string]TopologyHandler{},
}

var PhaseBeforeInstall = "BeforeInstall"
var PhaseBeforeUninstall = "BeforeUninstall"

type TopologyHandler interface {
	Generate() (Topology, error)
	Validate(topology Topology, phase string) (Topology, error)
	Install(topology Topology) (DeploymentOutput, error)
	Uninstall(topology Topology) (DeploymentOutput, error)
}

type AblePrintNotes interface {
	PrintNotes(topology Topology, deploymentOutput DeploymentOutput)
}

type TopologyHandlerManager struct {
	handlers map[string]TopologyHandler
}

func (t TopologyHandlerManager) AddHandler(kind string, handler TopologyHandler) {
	t.handlers[kind] = handler
}

// GetHandler returns nil on unsupported kind
func (t TopologyHandlerManager) GetHandler(kind string) TopologyHandler {
	return t.handlers[kind]
}
