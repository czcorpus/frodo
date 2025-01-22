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

func TestEnqueue(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}
	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	assert.Equal(t, &f1, q.firstEntry.job)
	assert.Equal(t, "1", q.firstEntry.initialState.GetID())
	assert.Equal(t, &f3, q.lastEntry.job)
	assert.Equal(t, "3", q.lastEntry.initialState.GetID())
	assert.Equal(t, 3, q.Size())
}

func TestDequeueOne(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}
	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	ans, st, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, &f1, ans)
	assert.Equal(t, "1", st.GetID())
	assert.Equal(t, 2, q.Size())
}

func TestDequeueAll(t *testing.T) {
	q := JobQueue{}
	var err error
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}

	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	_, _, err = q.Dequeue()
	assert.NoError(t, err)
	_, _, err = q.Dequeue()
	assert.NoError(t, err)
	var f *QueuedFunc
	var st GeneralJobInfo
	f, st, err = q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, &f3, f)
	assert.Equal(t, "3", st.GetID())
	assert.Equal(t, 0, q.Size())
}

func TestRepeatedlyEmptied(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}

	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Dequeue()
	q.Dequeue()
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	assert.Equal(t, 1, q.Size())
	assert.Equal(t, &f3, q.firstEntry.job)
	assert.Equal(t, "3", q.firstEntry.initialState.GetID())
	assert.Equal(t, &f3, q.lastEntry.job)
	assert.Equal(t, "3", q.lastEntry.initialState.GetID())
}

func TestDequeueOnEmpty(t *testing.T) {
	q := JobQueue{}
	_, _, err := q.Dequeue()
	assert.Equal(t, ErrorEmptyQueue, err)
}

func TestDelayNextOnEmpty(t *testing.T) {
	q := JobQueue{}
	err := q.DelayNext()
	assert.Equal(t, err, ErrorEmptyQueue)
}
func TestDelayNextOnTwoItemQueue(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	err := q.DelayNext()
	assert.NoError(t, err)
	assert.Equal(t, &f1, q.lastEntry.job)
	assert.Equal(t, &f2, q.firstEntry.job)
	v, st, err := q.Dequeue()
	assert.Equal(t, "2", st.GetID())
	assert.Equal(t, &f2, v)
	assert.NoError(t, err)
}
