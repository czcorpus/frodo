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
	"errors"
)

var (
	ErrorEmptyQueue = errors.New("empty queue")
)

type QueuedFunc = func(chan<- GeneralJobInfo)

type JobEntry struct {
	next         *JobEntry
	job          *QueuedFunc
	initialState GeneralJobInfo
}

type JobQueue struct {
	firstEntry *JobEntry
	lastEntry  *JobEntry
}

func (jq *JobQueue) Size() int {
	ans := 0
	for curr := jq.firstEntry; curr != nil; curr = curr.next {
		ans++
	}
	return ans
}

func (jq *JobQueue) Enqueue(item *QueuedFunc, initialState GeneralJobInfo) {
	entry := &JobEntry{
		job:          item,
		initialState: initialState,
	}
	if jq.firstEntry == nil {
		jq.firstEntry = entry
	}
	if jq.lastEntry == nil {
		jq.lastEntry = entry

	} else {
		jq.lastEntry.next = entry
	}
	jq.lastEntry = entry
}

func (jq *JobQueue) getPenultimate() *JobEntry {
	var prev *JobEntry
	for curr := jq.firstEntry; curr != nil && curr.next != nil; curr = curr.next {
		prev = curr
	}
	return prev
}

// DelayNext takes the current item to be dequeued and moves
// it one position back. In case the queue contains only a single
// item, the function does nothing. In case the queue is empty,
// ErrorEmptyQueue is returned.
func (jq *JobQueue) DelayNext() error {
	if jq.firstEntry == nil {
		return ErrorEmptyQueue
	}
	if jq.Size() == 2 {
		first := jq.firstEntry
		jq.firstEntry = jq.lastEntry
		jq.firstEntry.next = first
		first.next = nil
		jq.lastEntry = first

	} else if jq.Size() > 2 {
		pu := jq.getPenultimate()
		last := jq.lastEntry
		pu.next = nil
		jq.lastEntry = pu
		var pupu *JobEntry
		for pupu = jq.firstEntry; pupu != nil && pupu.next != pu; pupu = pupu.next {
		}
		pupu.next = last
		last.next = jq.lastEntry
	}
	return nil
}

func (jq *JobQueue) Dequeue() (*QueuedFunc, GeneralJobInfo, error) {
	ret := jq.firstEntry
	if ret == nil {
		return nil, nil, ErrorEmptyQueue
	}
	nxt := ret.next
	if nxt != nil {
		jq.firstEntry = nxt

	} else {
		jq.firstEntry = nil
		jq.lastEntry = nil
	}
	return ret.job, ret.initialState, nil
}

func (jq *JobQueue) PeekID() (string, error) {
	if jq.firstEntry == nil {
		return "", ErrorEmptyQueue
	}
	return jq.firstEntry.initialState.GetID(), nil
}
