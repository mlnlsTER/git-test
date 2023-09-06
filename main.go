package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const bufferDrainInterval time.Duration = 15 * time.Second

const bufferSize int = 10

type RingBuffer struct {
	size   int
	data   []*int
	Start  int
	End    int
	isFull bool
	m      sync.Mutex
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{size, make([]*int, size), 0, 0, false, sync.Mutex{}}
}

func (r *RingBuffer) Push(el *int) {
	r.m.Lock()
	defer r.m.Unlock()
	r.data[r.End] = el
	r.End = (r.End + 1) % r.size
	if r.End == r.Start {
		r.Start = (r.Start + 1) % r.size
		r.isFull = true
	}
}

func (r *RingBuffer) Get() []*int {
	r.m.Lock()
	defer r.m.Unlock()
	if r.Start == r.End && !r.isFull {
		return nil
	}
	var output []*int
	index := r.Start
	for {
		output = append(output, r.data[index])
		index = (index + 1) % r.size
		if index == r.End {
			break
		}
	}
	r.Start = r.End
	r.isFull = false
	return output
}

type StageInt func(<-chan bool, <-chan *int) <-chan *int

type PipeLine struct {
	stages []StageInt
	done   <-chan bool
}

func NewPipeline(done <-chan bool, stages ...StageInt) *PipeLine {
	return &PipeLine{done: done, stages: stages}
}

func (p *PipeLine) Run(source <-chan *int) <-chan *int {
	var c <-chan *int = source
	for index := range p.stages {
		c = p.runStage(p.stages[index], c)
	}
	return c
}

func (p *PipeLine) runStage(stage StageInt, sourceChan <-chan *int) <-chan *int {
	return stage(p.done, sourceChan)
}

func main() {
	dataSource := func() (<-chan *int, <-chan bool) {
		c := make(chan *int)
		done := make(chan bool)
		go func() {
			defer close(done)
			scanner := bufio.NewScanner(os.Stdin)
			var data string
			for {
				scanner.Scan()
				data = scanner.Text()
				if strings.EqualFold(data, "exit") {
					log.Println("Программа завершила работу!")
					return
				}
				i, err := strconv.Atoi(data)
				if err != nil {
					log.Println("Программа обрабатывает только целые числа!")
					continue
				}
				c <- &i
			}
		}()
		return c, done
	}
	negativeFilter := func(done <-chan bool, c <-chan *int) <-chan *int {
		convertedIntChan := make(chan *int)
		go func() {
			for {
				select {
				case data := <-c:
					if *data > 0 {
						log.Println("Успешно пройдена проверка фильтра отрицательных чисел:", *data)
						select {
						case convertedIntChan <- data:
						case <-done:
							return
						}
					} else {
						log.Println("Не пройдена проверка фильтра отрицательных чисел:", *data)
					}
				case <-done:
					return
				}
			}
		}()
		return convertedIntChan
	}

	NotMultipleOfThreeFilter := func(done <-chan bool, c <-chan *int) <-chan *int {
		filteredIntChan := make(chan *int)
		go func() {
			for {
				select {
				case data := <-c:
					if *data != 0 && *data%3 == 0 {
						log.Println("Успешно пройдена проверка фильтра чисел, не кратных 3:", *data)
						select {
						case filteredIntChan <- data:
						case <-done:
							return
						}
					} else {
						log.Println("Не пройдена проверка фильтра чисел, не кратных 3:", *data)
					}
				case <-done:
					return
				}

			}
		}()
		return filteredIntChan
	}

	buffer := func(done <-chan bool, c <-chan *int) <-chan *int {
		bufferedIntChan := make(chan *int)
		ringBuffer := NewRingBuffer(bufferSize)
		go func() {
			for {
				select {
				case data := <-c:
					ringBuffer.Push(data)
				case <-done:
					return
				}
			}
		}()

		go func() {
			for {
				select {
				case <-time.After(bufferDrainInterval):
					bufferData := ringBuffer.Get()
					if bufferData != nil {
						for _, data := range bufferData {
							select {
							case bufferedIntChan <- data:
							case <-done:
								return
							}
						}
					}
				case <-done:
					return
				}
			}
		}()
		return bufferedIntChan
	}

	consumer := func(done <-chan bool, c <-chan *int) {
		for {
			select {
			case data := <-c:
				log.Printf("Обработаны данные: %d\n", *data)
			case <-done:
				return
			}
		}
	}
	source, done := dataSource()
	pipeline := NewPipeline(done, negativeFilter, NotMultipleOfThreeFilter, buffer)
	consumer(done, pipeline.Run(source))
}
