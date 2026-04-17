package yatgbot

import "sync"

const uniqueStringsNoopThreshold = 2

type asyncUpdateScheduler struct {
	mu        sync.Mutex
	jobs      []*asyncUpdateJob
	keyQueues map[string][]*asyncUpdateJob
	busyKeys  map[string]struct{}
}

type asyncUpdateJob struct {
	keys    []string
	run     func()
	started bool
	done    bool
}

func newAsyncUpdateScheduler() *asyncUpdateScheduler {
	return &asyncUpdateScheduler{
		keyQueues: make(map[string][]*asyncUpdateJob),
		busyKeys:  make(map[string]struct{}),
	}
}

func (s *asyncUpdateScheduler) Enqueue(keys []string, run func()) {
	if run == nil {
		return
	}

	keys = uniqueStrings(keys)

	if len(keys) == 0 {
		go run()

		return
	}

	job := &asyncUpdateJob{
		keys: keys,
		run:  run,
	}

	s.mu.Lock()
	s.jobs = append(s.jobs, job)

	for _, key := range keys {
		s.keyQueues[key] = append(s.keyQueues[key], job)
	}

	runnable := s.collectRunnableLocked()
	s.mu.Unlock()

	s.start(runnable)
}

func (s *asyncUpdateScheduler) collectRunnableLocked() []*asyncUpdateJob {
	runnable := make([]*asyncUpdateJob, 0)

	for _, job := range s.jobs {
		if job.started || job.done || !s.canStartLocked(job) {
			continue
		}

		job.started = true

		for _, key := range job.keys {
			s.busyKeys[key] = struct{}{}
		}

		runnable = append(runnable, job)
	}

	return runnable
}

func (s *asyncUpdateScheduler) canStartLocked(job *asyncUpdateJob) bool {
	for _, key := range job.keys {
		if _, busy := s.busyKeys[key]; busy {
			return false
		}

		queue := s.keyQueues[key]
		if len(queue) == 0 || queue[0] != job {
			return false
		}
	}

	return true
}

func (s *asyncUpdateScheduler) finish(job *asyncUpdateJob) {
	s.mu.Lock()
	job.done = true

	for _, key := range job.keys {
		delete(s.busyKeys, key)

		queue := s.keyQueues[key]
		if len(queue) == 0 {
			continue
		}

		if queue[0] == job {
			queue = queue[1:]
		} else {
			for i, queued := range queue {
				if queued != job {
					continue
				}

				queue = append(queue[:i], queue[i+1:]...)

				break
			}
		}

		if len(queue) == 0 {
			delete(s.keyQueues, key)

			continue
		}

		s.keyQueues[key] = queue
	}

	for len(s.jobs) > 0 && s.jobs[0].done {
		s.jobs = s.jobs[1:]
	}

	runnable := s.collectRunnableLocked()
	s.mu.Unlock()

	s.start(runnable)
}

func (s *asyncUpdateScheduler) start(jobs []*asyncUpdateJob) {
	for _, job := range jobs {
		current := job

		go func() {
			defer s.finish(current)

			current.run()
		}()
	}
}

func uniqueStrings(values []string) []string {
	if len(values) < uniqueStringsNoopThreshold {
		return values
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
