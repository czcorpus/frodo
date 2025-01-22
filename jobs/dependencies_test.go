// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddDependency(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("child1", "parent1")
	assert.Equal(t, deps["child1"][0].jobID, "parent1")
}

func TestCheckDependencies(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("child1", "parent1")
	deps.Add("child1", "parent2")
	ans, err := deps.MustWait("child1")
	assert.NoError(t, err)
	assert.True(t, ans)
	ans, err = deps.HasFailedParent("child1")
	assert.NoError(t, err)
	assert.False(t, ans)
}

func TestFinishedParentUnblocking(t *testing.T) {
	deps := make(JobsDeps)
	err := deps.Add("child1", "parentA")
	assert.NoError(t, err)
	err = deps.Add("child1", "parentB")
	assert.NoError(t, err)
	err = deps.Add("child2", "parentA")
	assert.NoError(t, err)
	deps.SetParentFinished("parentA", false)

	ans, err := deps.MustWait("child1")
	assert.NoError(t, err)
	assert.True(t, ans) // because child1 has yet another dependency

	ans, err = deps.MustWait("child2")
	assert.NoError(t, err)
	assert.False(t, ans)
}

func TestFailedParent(t *testing.T) {
	deps := make(JobsDeps)
	err := deps.Add("child1", "parentA")
	assert.NoError(t, err)
	err = deps.Add("child1", "parentB")
	assert.NoError(t, err)
	deps.SetParentFinished("parentA", true)

	ans, err := deps.MustWait("child1")
	assert.NoError(t, err)
	assert.False(t, ans)
	ans, err = deps.HasFailedParent("child1")
	assert.NoError(t, err)
	assert.True(t, ans)
}

func TestCannotCreateCircle(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("item1", "item1_1")
	deps.Add("item2", "parent2")
	deps.Add("item3", "parent2")
	deps.Add("item1_1", "item1_1_1")
	deps.Add("item1_1", "item1_1_2")
	deps.Add("item1_1_1", "item2")
	deps.Add("item1_1_2", "item1_1_2_1")
	err := deps.Add("item2", "item1")
	assert.Equal(t, ErrorCircularJobDependency, err)
}

func TestFindTrivialCircle(t *testing.T) {
	deps := make(JobsDeps)
	err := deps.Add("item1", "item1")
	assert.Equal(t, ErrorCircularJobDependency, err)
}

func TestCannotRepeatParent(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("item1", "item2")
	deps.Add("item1", "item3")
	err := deps.Add("item1", "item2")
	assert.Equal(t, ErrorDuplicateDependency, err)
	assert.Equal(t, 2, len(deps["item1"]))
}
