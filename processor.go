package pipz

import (
	"sync"
)

type Processor[T any] func(T) (T, error)

type Pipeline[T any] struct {
	processors []Processor[T]
	mu         sync.RWMutex
}

func NewPipeline[T any]() *Pipeline[T] {
	return &Pipeline[T]{
		processors: make([]Processor[T], 0),
	}
}

func (p *Pipeline[T]) Register(processors ...Processor[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processors = append(p.processors, processors...)
}

func (p *Pipeline[T]) Process(input T) (T, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var err error
	for _, processor := range p.processors {
		input, err = processor(input)
		if err != nil {
			return input, err
		}
	}
	return input, nil
}
