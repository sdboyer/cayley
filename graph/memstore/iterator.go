// Copyright 2014 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memstore

import (
	"fmt"
	"math"
	"strings"

	"github.com/google/cayley/graph"
	"github.com/google/cayley/graph/iterator"
	"github.com/google/cayley/graph/memstore/b"
)

type Iterator struct {
	uid    uint64
	ts     *TripleStore
	tags   graph.Tagger
	tree   *b.Tree
	iter   *b.Enumerator
	data   string
	result graph.Value
}

func cmp(a, b int64) int {
	return int(a - b)
}

func NewIterator(tree *b.Tree, data string, ts *TripleStore) *Iterator {
	iter, err := tree.SeekFirst()
	if err != nil {
		iter = nil
	}
	return &Iterator{
		uid:  iterator.NextUID(),
		ts:   ts,
		tree: tree,
		iter: iter,
		data: data,
	}
}

func (it *Iterator) UID() uint64 {
	return it.uid
}

func (it *Iterator) Reset() {
	var err error
	it.iter, err = it.tree.SeekFirst()
	if err != nil {
		it.iter = nil
	}
}

func (it *Iterator) Tagger() *graph.Tagger {
	return &it.tags
}

func (it *Iterator) TagResults(dst map[string]graph.Value) {
	for _, tag := range it.tags.Tags() {
		dst[tag] = it.Result()
	}

	for tag, value := range it.tags.Fixed() {
		dst[tag] = value
	}
}

func (it *Iterator) Clone() graph.Iterator {
	var iter *b.Enumerator
	if it.result != nil {
		var ok bool
		iter, ok = it.tree.Seek(it.result.(int64))
		if !ok {
			panic("value unexpectedly missing")
		}
	} else {
		var err error
		iter, err = it.tree.SeekFirst()
		if err != nil {
			iter = nil
		}
	}

	m := &Iterator{
		uid:  iterator.NextUID(),
		ts:   it.ts,
		tree: it.tree,
		iter: iter,
		data: it.data,
	}
	m.tags.CopyFrom(it)

	return m
}

func (it *Iterator) Close() {}

func (it *Iterator) checkValid(index int64) bool {
	return it.ts.log[index].DeletedBy == 0
}

func (it *Iterator) Next() bool {
	graph.NextLogIn(it)

	if it.iter == nil {
		return graph.NextLogOut(it, nil, false)
	}
	result, _, err := it.iter.Next()
	if err != nil {
		return graph.NextLogOut(it, nil, false)
	}
	if !it.checkValid(result) {
		return it.Next()
	}
	it.result = result
	return graph.NextLogOut(it, it.result, true)
}

func (it *Iterator) ResultTree() *graph.ResultTree {
	return graph.NewResultTree(it.Result())
}

func (it *Iterator) Result() graph.Value {
	return it.result
}

func (it *Iterator) NextPath() bool {
	return false
}

// No subiterators.
func (it *Iterator) SubIterators() []graph.Iterator {
	return nil
}

func (it *Iterator) Size() (int64, bool) {
	return int64(it.tree.Len()), true
}

func (it *Iterator) Contains(v graph.Value) bool {
	graph.ContainsLogIn(it, v)
	if _, ok := it.tree.Get(v.(int64)); ok {
		it.result = v
		return graph.ContainsLogOut(it, v, true)
	}
	return graph.ContainsLogOut(it, v, false)
}

func (it *Iterator) Seek(v graph.Value) bool {
	graph.SeekLogIn(it, v)

	iter, found := it.tree.Seek(v.(int64))
	if found {
		it.iter = iter
	}

	return graph.SeekLogOut(it, v, found)
}

func (it *Iterator) DebugString(indent int) string {
	size, _ := it.Size()
	return fmt.Sprintf("%s(%s tags:%s size:%d %s)", strings.Repeat(" ", indent), it.Type(), it.tags.Tags(), size, it.data)
}

var memType graph.Type

func init() {
	memType = graph.RegisterIterator("b+tree")
}

func Type() graph.Type { return memType }

func (it *Iterator) Type() graph.Type { return memType }

func (it *Iterator) Sorted() bool { return true }

func (it *Iterator) Optimize() (graph.Iterator, bool) {
	return it, false
}

func (it *Iterator) Stats() graph.IteratorStats {
	return graph.IteratorStats{
		ContainsCost: int64(math.Log(float64(it.tree.Len()))) + 1,
		NextCost:     1,
		Size:         int64(it.tree.Len()),
	}
}
