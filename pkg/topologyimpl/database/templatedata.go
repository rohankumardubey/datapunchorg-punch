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

package database

import (
	"github.com/datapunchorg/punch/pkg/framework"
)

type DatabaseTemplateData struct {
	framework.TemplateDataWithRegion
}

func CreateDatabaseTemplateData(data framework.TemplateData) DatabaseTemplateData {
	copy := framework.CopyTemplateData(data)
	result := DatabaseTemplateData{
		TemplateDataWithRegion: framework.TemplateDataWithRegion{
			TemplateDataImpl: copy,
		},
	}
	return result
}


