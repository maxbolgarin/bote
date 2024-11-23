package bote

import (
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/exp/constraints"
)

const stackDefaultCap = 8

type stackImpl[T comparable] struct {
	mem []T
	ind map[T]int
	mu  sync.Mutex
}

func newStack[T comparable](d ...[]T) *stackImpl[T] {
	if len(d) > 0 {
		mem := append(make([]T, 0, getMax(len(d[0]), stackDefaultCap)), d[0]...)
		ind := make(map[T]int, getMax(len(d[0]), stackDefaultCap))
		for i, v := range d[0] {
			ind[v] = i
		}
		return &stackImpl[T]{mem: mem, ind: ind}
	}
	return &stackImpl[T]{mem: make([]T, 0, stackDefaultCap), ind: make(map[T]int, stackDefaultCap)}
}

func (s *stackImpl[T]) Push(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index, ok := s.ind[item]; ok {
		last := len(s.mem) - 1
		if index == last {
			return
		}
		s.ind[s.mem[last]] = index
		s.ind[item] = last

		s.mem[index], s.mem[last] = s.mem[last], s.mem[index]
		return
	}
	s.ind[item] = len(s.mem)
	s.mem = append(s.mem, item)
}

func (s *stackImpl[T]) Last() T {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.mem) == 0 {
		var zero T
		return zero
	}

	return s.mem[len(s.mem)-1]
}

func (s *stackImpl[T]) Pop() T {
	m, _ := s.PopOK()
	return m
}

func (s *stackImpl[T]) PopOK() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.mem) == 0 {
		var zero T
		return zero, false
	}
	index := len(s.mem) - 1

	item := s.mem[index]
	s.mem = s.mem[:index]
	delete(s.ind, item)

	return item, true
}

func (s *stackImpl[T]) IsEmpty() bool {
	return s.Len() == 0
}

func (s *stackImpl[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.mem)
}

func (s *stackImpl[T]) Remove(item T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	indexToRemove, ok := s.ind[item]
	if !ok {
		return false
	}

	if indexToRemove >= len(s.mem) {
		s.ind = make(map[T]int, stackDefaultCap)
		s.mem = make([]T, 0, stackDefaultCap)
		panic("impossible: index in map can't be greater than memory length, clean the stack to continue working")
	}

	delete(s.ind, item)

	for item, ind := range s.ind {
		if ind < indexToRemove {
			continue
		}
		if ind > indexToRemove {
			s.ind[item] = ind - 1
		}
	}

	if indexToRemove == len(s.mem)-1 {
		s.mem = s.mem[:indexToRemove]
		return true
	}

	if indexToRemove == 0 {
		s.mem = s.mem[1:]
		return true
	}

	s.mem = append(s.mem[:indexToRemove], s.mem[indexToRemove+1:]...)
	return true
}

func (s *stackImpl[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = make([]T, 0, stackDefaultCap)
	s.ind = make(map[T]int, stackDefaultCap)
}

func (s *stackImpl[T]) Raw() []T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mem
}

func (i *stackImpl[T]) UnmarshalBSONValue(t bsontype.Type, value []byte) error {
	mem := make([]T, 0, stackDefaultCap)
	ind := make(map[T]int, stackDefaultCap)

	var res primitive.D
	if err := bson.Unmarshal(value, &res); err != nil {
		return err
	}

	if len(res) == 0 {
		return nil
	}

	for _, v := range res {
		vv, ok := v.Value.(primitive.D)
		if !ok {
			return fmt.Errorf("cannot cast %T to privitive.D", v.Value)
		}
		if len(vv) == 0 {
			continue
		}

		b, err := bson.Marshal(vv)
		if err != nil {
			return err
		}

		var out T

		if err = bson.Unmarshal(b, &out); err != nil {
			return err
		}

		mem = append(mem, out)
	}

	for index, item := range mem {
		ind[item] = index
	}

	*i = stackImpl[T]{
		mem: mem,
		ind: ind,
	}

	return nil
}

type Number interface {
	constraints.Integer | constraints.Float
}

func getMax[T Number](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func getMin[T Number](a, b T) T {
	if a < b {
		return a
	}
	return b
}
