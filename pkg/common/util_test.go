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

package common

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRetryUntilTrue(t *testing.T) {
	RetryUntilTrue(func() (bool, error) {
		return true, nil
	},
		0*time.Millisecond,
		0*time.Millisecond)
}

type struct1 struct {
	Id string
	Enabled bool
	Count int
}

func TestPatchStructFieldByStringValue(t *testing.T) {
	s1 := struct1{}
	var pointer interface{} = &s1
	err := PatchStructFieldByStringValue(pointer, "Id123", "new_value")
	assert.NotNil(t, err)
	err = PatchStructFieldByStringValue(pointer, "Id", "new_value")
	assert.Nil(t, err)
	assert.Equal(t, "new_value", s1.Id)
	err = PatchStructFieldByStringValue(pointer, "Enabled", "true")
	assert.Nil(t, err)
	assert.Equal(t, true, s1.Enabled)
	err = PatchStructFieldByStringValue(pointer, "Enabled", "false")
	assert.Nil(t, err)
	assert.Equal(t, false, s1.Enabled)
	err = PatchStructFieldByStringValue(pointer, "Count", "100")
	assert.Nil(t, err)
	assert.Equal(t, 100, s1.Count)
}
