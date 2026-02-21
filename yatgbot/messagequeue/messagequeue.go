package messagequeue

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgmessageencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// Dispatcher handles message sending with priority and concurrency control.
type Dispatcher struct {
	client              *yatgclient.Client
	storage             yatgstorage.IStorage
	messageQueueChannel chan MessageJob
	heap                messageHeap
	cond                sync.Cond
	parseMode           yatgmessageencoding.MessageEncoding
	log                 yalogger.Logger
}

// NewDispatcher creates a new Dispatcher with the given number of workers.
// Each worker processes jobs from the priority queue channel.
// The dispatcher uses a condition variable to signal when new jobs are added to the heap.
// It also initializes the message heap and starts the worker goroutines.
//
// Example usage:
//
// dispatcher := NewDispatcher(ctx, client, storage, workerCount, parseMode, log)
func NewDispatcher(
	ctx context.Context,
	client *yatgclient.Client,
	storage yatgstorage.IStorage,
	workerCount uint,
	parseMode yatgmessageencoding.MessageEncoding,
	log yalogger.Logger,
) *Dispatcher {
	dispatcher := &Dispatcher{
		parseMode:           parseMode,
		client:              client,
		storage:             storage,
		messageQueueChannel: make(chan MessageJob),
		log:                 log,
		heap:                newMessageHeap(),
		cond:                *sync.NewCond(&sync.Mutex{}),
	}

	go dispatcher.proccessMessagesQueue()

	for i := range workerCount {
		go dispatcher.worker(ctx, i)
	}

	return dispatcher
}

// DeleteJob removes a job from the heap by its ID.
// Returns true if the job was found and deleted, false otherwise.
//
// Example usage:
//
//	deleted := dispatcher.DeleteJob(jobID)
//
//	if !deleted {
//	    // Handle job not found
//	}
func (d *Dispatcher) DeleteJob(id uint64) bool {
	d.cond.L.Lock()
	defer d.cond.L.Unlock()

	return d.heap.Delete(id)
}

// DeleteJobFunc removes jobs from the heap that satisfy the given condition.
//
// Example usage:
//
//	deletedIDs := dispatcher.DeleteJobFunc(func(job MessageJob) bool {
//	    return job.Priority < 10
//	})
//
//	for _, id := range deletedIDs {
//		// Handle deleted job ID
//	}
func (d *Dispatcher) DeleteJobFunc(deleteFunc func(MessageJob) bool) []uint64 {
	d.cond.L.Lock()
	defer d.cond.L.Unlock()

	return d.heap.DeleteFunc(deleteFunc)
}

// AddRawJob adds a raw job to the dispatcher with the specified request, priority, and task count.
// It returns the job ID and a channel to receive the job result.
//
// Example usage:
//
// jobID, resultCh := dispatcher.AddRawJob(request, priority, taskCount)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) AddRawJob(
	request bin.Encoder,
	priority uint16,
	taskCount uint,
) (uint64, <-chan JobResult) {
	job := MessageJob{
		ID:        rand.Uint64(), //nolint:gosec,lll // We don't care about randomness quality here, as this is just for generating a random job ID
		Priority:  priority,
		Request:   request,
		ResultCh:  make(chan JobResult, 1),
		Timestamp: time.Now(),
		TaskCount: taskCount,
	}

	d.cond.L.Lock()

	d.heap.Push(job)

	d.cond.Signal()
	d.cond.L.Unlock()

	return job.ID, job.ResultCh
}

// AddEmptyJob adds the specified number of placeholder jobs to the dispatcher.
//
// Example usage:
//
// dispatcher.AddEmptyJob(5) // Adds 5 placeholder jobs
func (d *Dispatcher) AddEmptyJob(count uint) {
	for range count {
		d.heap.Push(MessageJob{
			IsPlaceholder: true,
		})
	}
}

// AddForwardMessagesJob adds a message forwarding job to the dispatcher.
//
// Example usage:
//
// jobID, resultCh := dispatcher.AddForwardMessagesJob(ctx, messagesForwardMessagesRequest, priority)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) AddForwardMessagesJob(
	ctx context.Context,
	req *tg.MessagesForwardMessagesRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	fromPeer, err := d.ensurePeerAccessHash(ctx, req.FromPeer)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.FromPeer = fromPeer

	toPeer, err := d.ensurePeerAccessHash(ctx, req.ToPeer)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.ToPeer = toPeer

	sendAsPeer, err := d.ensurePeerAccessHash(ctx, req.SendAs)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.SendAs = sendAsPeer

	req.RandomID = make([]int64, len(req.ID))
	for i := range req.RandomID {
		req.RandomID[i] = rand.Int63() //nolint:gosec,lll // We don't care about randomness quality here, as this is just for generating random IDs for forwarding messages
	}

	return d.AddRawJob(req, priority, uint(len(req.RandomID)))
}

// AddSendMessageJob adds a message sending job to the dispatcher.
//
// Example usage:
//
// jobID, resultCh := dispatcher.AddSendMessageJob(ctx, messagesSendMessageRequest, priority)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) AddSendMessageJob(
	ctx context.Context,
	req *tg.MessagesSendMessageRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	peer, err := d.ensurePeerAccessHash(ctx, req.Peer)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.Peer = peer

	sendAsPeer, err := d.ensurePeerAccessHash(ctx, req.SendAs)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.SendAs = sendAsPeer

	if d.parseMode != nil {
		message, entities := d.parseMode.Parse(req.Message)

		req.Message = message
		req.Entities = entities
	}

	if req.RandomID == 0 {
		req.RandomID = rand.Int63() //nolint:gosec,lll // We don't care about randomness quality here, as this is just for generating a random ID for sending a message
	}

	return d.AddRawJob(req, priority, SingleMessage)
}

// AddSendMultiMediaJob adds a media sending job to the dispatcher.
//
// Example usage:
//
// jobID, resultCh := dispatcher.AddSendMultiMediaJob(ctx, messagesSendMediaRequest, priority)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) AddSendMultiMediaJob(
	ctx context.Context,
	req *tg.MessagesSendMultiMediaRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	preparedPeer, err := d.ensurePeerAccessHash(ctx, req.Peer)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.Peer = preparedPeer

	sendAsPeer, err := d.ensurePeerAccessHash(ctx, req.SendAs)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.SendAs = sendAsPeer

	if d.parseMode != nil {
		for i, media := range req.MultiMedia {
			message, entities := d.parseMode.Parse(media.Message)

			media.Message = message
			media.Entities = entities

			req.MultiMedia[i] = media
		}
	}

	for i := range req.MultiMedia {
		req.MultiMedia[i].RandomID = rand.Int63() //nolint:gosec,lll // We don't care about randomness quality here, as this is just for generating random IDs for sending media
	}

	return d.AddRawJob(req, priority, uint(len(req.MultiMedia)))
}

// AddSendMediaJob adds a media sending job to the dispatcher.
//
// Example usage:
//
// jobID, resultCh := dispatcher.AddSendMediaJob(ctx, messagesSendMediaRequest, priority)
//
// // Wait for the job result
//
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) AddSendMediaJob(
	ctx context.Context,
	req *tg.MessagesSendMediaRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	preparedPeer, err := d.ensurePeerAccessHash(ctx, req.Peer)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.Peer = preparedPeer

	sendAsPeer, err := d.ensurePeerAccessHash(ctx, req.SendAs)
	if err != nil {
		return 0, returnErrorJobResult(err)
	}

	req.SendAs = sendAsPeer

	if d.parseMode != nil {
		message, entities := d.parseMode.Parse(req.Message)

		req.Message = message
		req.Entities = entities
	}

	if req.RandomID == 0 {
		req.RandomID = rand.Int63() //nolint:gosec,lll // We don't care about randomness quality here, as this is just for generating a random ID for sending a message
	}

	return d.AddRawJob(req, priority, SingleMessage)
}

// proccessMessagesQueue continuously processes jobs from the heap and sends them to the priority queue channel.
// It waits for new jobs if the heap is empty.
func (d *Dispatcher) proccessMessagesQueue() {
	for {
		d.cond.L.Lock()

		if d.heap.Len() == 0 {
			d.cond.Wait()
		}

		job, ok := d.heap.Pop()
		d.cond.L.Unlock()

		if !ok {
			continue
		}

		d.messageQueueChannel <- job
	}
}

// worker processes jobs from the priority queue channel.
// It executes each job and sends the result back through the job's ResultCh.
func (d *Dispatcher) worker(ctx context.Context, id uint) {
	for {
		select {
		case job := <-d.messageQueueChannel:
			start := time.Now()

			jobResult := job.Execute(ctx, d, id)

			select {
			case job.ResultCh <- jobResult:
			case <-ctx.Done():
				return
			}

			time.Sleep(time.Second - time.Since(start))

		case <-ctx.Done():
			return
		}
	}
}

// ensurePeerAccessHash checks if the given peer has an access hash.
// If not, it retrieves the access hash from storage and updates the peer.
func (d *Dispatcher) ensurePeerAccessHash(
	ctx context.Context,
	peer tg.InputPeerClass,
) (tg.InputPeerClass, yaerrors.Error) {
	bot, err := d.client.Self(ctx)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to get self for preparePeer",
		)
	}

	switch p := peer.(type) {
	case *tg.InputPeerUser:
		if p.AccessHash == 0 || p.UserID != 0 {
			accessHash, err := d.storage.GetUserAccessHash(ctx, bot.ID, p.UserID)
			if err != nil {
				return nil, err.Wrap("failed to get user access hash for ensurePeerAccessHash")
			}

			p.AccessHash = accessHash

			return p, nil
		}

		return peer, nil
	case *tg.InputPeerChannel:
		if p.AccessHash == 0 || p.ChannelID != 0 {
			accessHash, found, err := d.storage.GetChannelAccessHash(ctx, bot.ID, p.ChannelID)
			if err != nil {
				return nil, err.Wrap("failed to get channel access hash for ensurePeerAccessHash")
			}

			if !found {
				return nil, yaerrors.FromString(
					http.StatusNotFound,
					fmt.Sprintf("access hash for channel %d not found", p.ChannelID),
				)
			}

			p.AccessHash = accessHash

			return p, nil
		}

		return peer, nil
	default:
		return peer, nil
	}
}

// returnErrorJobResult creates a channel that immediately returns a JobResult
// with the given error and then closes the channel.
func returnErrorJobResult(err yaerrors.Error) <-chan JobResult {
	ch := make(chan JobResult, 1)
	ch <- JobResult{
		Err: err,
	}

	close(ch)

	return ch
}
