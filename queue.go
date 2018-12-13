package main

import (
	"os"
	"sync"
)

type Queue struct {
	data []os.FileInfo
	i    int
	m    sync.Mutex
}

func NewQueue(data []os.FileInfo) *Queue {
	q := Queue{
		data: data,
		i:    0,
		m:    sync.Mutex{}}
	return &q
}

//Next pumps next element of queue
func (q *Queue) Next() (*os.FileInfo, bool) {
	q.m.Lock()
	defer q.m.Unlock()

	if q.i < len(q.data) {
		val := q.data[q.i]
		q.i++
		return &val, true
	}

	return nil, false
}
