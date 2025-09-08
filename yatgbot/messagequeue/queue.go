package messagequeue

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

type Dispatcher struct {
	mu                   sync.Mutex
	inputChannel         chan MessageJob
	priorityQueueChannel chan MessageJob
	wg                   sync.WaitGroup
	log                  yalogger.Logger
}

func NewDispatcher(ctx context.Context, workerCount int, log yalogger.Logger) *Dispatcher {
	dispatcher := &Dispatcher{
		inputChannel:         make(chan MessageJob, 100),
		priorityQueueChannel: make(chan MessageJob, 100),
		log:                  log,
	}

	dispatcher.wg.Add(1)
	go dispatcher.reorderMessages(ctx)

	for i := 0; i < workerCount; i++ {
		dispatcher.wg.Add(1)
		go dispatcher.worker(ctx, i)
	}

	return dispatcher
}

func (d *Dispatcher) reorderMessages(ctx context.Context) {
	defer d.wg.Done()

	var pq messageHeap

	heap.Init(&pq)

	for {
		select {
		case job := <-d.inputChannel:
			heap.Push(&pq, job)
		case <-ctx.Done():
			return
		}

		if pq.Len() > 0 {
			job := heap.Pop(&pq).(MessageJob)
			d.priorityQueueChannel <- job
		}
	}
}

func (d *Dispatcher) worker(ctx context.Context, id int) {
	defer d.wg.Done()

	for {
		select {
		case job := <-d.priorityQueueChannel:
			start := time.Now()

			if job.Markup == nil {
				if _, err := job.Sender.To(job.To).Text(ctx, job.Text); err != nil {
					d.log.Infof("[worker %d] error sending message without markup: %v", id, err)
				}
			} else {
				if _, err := job.Sender.To(job.To).Markup(job.Markup).Text(ctx, job.Text); err != nil {
					d.log.Infof("[worker %d] error sending message with markup: %v", id, err)
				}
			}

			elapsed := time.Since(start)
			if wait := time.Second - elapsed; wait > 0 {
				time.Sleep(wait)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) Add(job MessageJob) {
	d.inputChannel <- job
}
