// Copyright 2018 The go-contatract Authors
// This file is part of the go-contatract library.
//
// The go-contatract library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-contatract library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-contatract library. If not, see <http://www.gnu.org/licenses/>.

package queue

//import "fmt"

type Element interface{}

type Fifo interface {
	Put(e Element) //向队列中添加元素
	Get() Element  //移除队列中最前面的元素
	Clear() bool   //清空队列
	Size() int     //获取队列的元素个数
	IsEmpty() bool //判断队列是否是空
}

type SliceFifo struct {
	element []Element
}

func NewQueue() *SliceFifo {
	return &SliceFifo{}
}

//向队列中添加元素
func (f *SliceFifo) Put(e Element) {
	f.element = append(f.element, e)
}

func (f *SliceFifo) Peek() Element {
	if f.IsEmpty() {
		//fmt.Println("queue is empty!")
		return nil
	}
	firstElement := f.element[0]
	return firstElement
}

//移除队列中最前面的额元素
func (f *SliceFifo) Get() Element {
	if f.IsEmpty() {
		//fmt.Println("queue is empty!")
		return nil
	}
	firstElement := f.element[0]
	f.element = f.element[1:]
	return firstElement
}

func (f *SliceFifo) Clear() bool {
	if f.IsEmpty() {
		//fmt.Println("queue is empty!")
		return false
	}
	for i := 0; i < f.Size(); i++ {
		f.element[i] = nil
	}
	f.element = nil
	return true
}

func (f *SliceFifo) Size() int {
	return len(f.element)
}

func (f *SliceFifo) IsEmpty() bool {
	if len(f.element) == 0 {
		return true
	}
	return false
}

func (f *SliceFifo) GetData(i uint32) Element {
	if i >= uint32(len(f.element)) {
		return nil
	}
	return f.element[i]
}
