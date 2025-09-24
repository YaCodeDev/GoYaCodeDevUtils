package messagequeue

import (
	"time"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

type MessageJob struct {
	Priority  uint16
	Timestamp time.Time
	Text      string
	Markup    tg.ReplyMarkupClass
	Sender    *message.Sender
	To        tg.InputPeerClass
}

type messageHeap []MessageJob

func (h messageHeap) Len() int { return len(h) }

func (h messageHeap) Less(i int, j int) bool {
	if h[i].Priority == h[j].Priority {
		return h[i].Timestamp.Before(h[j].Timestamp)
	}

	return h[i].Priority < h[j].Priority
}

func (h messageHeap) Swap(i int, j int) { h[i], h[j] = h[j], h[i] }

func (h *messageHeap) Push(x any) {
	job, ok := x.(MessageJob)
	if !ok {
		return
	}

	*h = append(*h, job)
}

func (h *messageHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]

	return x
}
