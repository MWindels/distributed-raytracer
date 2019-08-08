// Package pool provides a worker pool object for use by the master.
package pool

import (
	"github.com/mwindels/distributed-raytracer/shared/comms"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"context"
	"sync"
	"time"
	"log"
	"fmt"
)

// HeartbeatFrequency controls how often heartbeats are sent to each worker in a pool.
const HeartbeatFrequency uint = 500

// HeartbeatTimeout controls how long heartbeats are waited on before the associated worker is assumed to be disconnected.
const HeartbeatTimeout uint = 1000

// worker represents an entry in a pool.
type worker struct {
	connection *grpc.ClientConn
	stopHeartbeats chan struct{}
	closing bool
	
	tasks uint
	index uint
}

// Pool represents a threadsafe worker pool.
type Pool struct {
	mu sync.RWMutex
	heap []*worker
	addresses map[string]*worker
}

// NewPool creates a new worker pool with a given initial capacity.
func NewPool(c uint) Pool {
	return Pool{
		mu: sync.RWMutex{},
		heap: make([]*worker, 0, c),
		addresses: make(map[string]*worker),
	}
}

// Destroy cleans up a worker pool.
func (p *Pool) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Close all the open connections.
	for a, w := range p.addresses {
		p.remove(a, w)
	}
}

// Size returns the number of workers in the pool.
func (p *Pool) Size() uint {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return uint(len(p.heap))
}

// swap swaps two workers in the heap.
// This function assumes that the heap has already been locked.
func (p *Pool) swap(i, j uint) {
	if i < uint(len(p.heap)) && j < uint(len(p.heap)) {
		// Swap the elements.
		temp := p.heap[i]
		p.heap[i] = p.heap[j]
		p.heap[j] = temp
		
		// Update their indices.
		p.heap[i].index = uint(i)
		p.heap[j].index = uint(j)
	}
}

// bubbleUp pushes a worker up the heap as long as it has fewer tasks than its parent.
// This function assumes that the heap has already been locked.
func (p *Pool) bubbleUp(w *worker) {
	if w != nil {
		if w.index < uint(len(p.heap)) && p.heap[w.index] == w {
			
			// While the worker has a parent...
			for i := w.index; i > 0; {
				parent := i / 2
				
				// If the worker has fewer tasks than its parent, bubble up.
				if p.heap[i].tasks < p.heap[parent].tasks {
					p.swap(i, parent)
					i = parent
				}else{
					break
				}
			}
		}
	}
}

// bubbleDown pushes a worker down the heap as long as it has more tasks than one of its children.
// This function assumes that the heap has already been locked.
func (p *Pool) bubbleDown(w *worker) {
	if w != nil {
		if w.index < uint(len(p.heap)) && p.heap[w.index] == w {
			
			// While the worker has at least one child...
			for i := w.index; 2 * i + 1 < uint(len(p.heap)); {
				left := 2 * i + 1
				if 2 * i + 2 < uint(len(p.heap)) {
					right := 2 * i + 2
					
					// The worker has two children, so compare against the child with with fewer tasks.
					if p.heap[left].tasks <= p.heap[right].tasks {
						// If the worker has more tasks than its left child, bubble down.
						if p.heap[i].tasks > p.heap[left].tasks {
							p.swap(i, left)
							i = left
						}else{
							break
						}
					}else{
						// If the worker has more tasks than its right child, bubble down.
						if p.heap[i].tasks > p.heap[right].tasks {
							p.swap(i, right)
							i = right
						}else{
							break
						}
					}
				}else{
					// If the worker has more tasks than its left child, bubble down.
					if p.heap[i].tasks > p.heap[left].tasks {
						p.swap(i, left)
						i = left
					}else{
						break
					}
				}
			}
		}
	}
}

// Assign assigns a task to the worker who is the least busy.
func (p *Pool) Assign(order *comms.WorkOrder, timeout uint) (<-chan *comms.TraceResults, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if len(p.heap) > 0 {
		resultsCh := make(chan *comms.TraceResults)
		assignee := p.heap[0]
		
		// Assign the task and re-arrange the heap.
		assignee.tasks += 1
		p.bubbleDown(assignee)
		
		// Perform the task.
		go func(out chan<- *comms.TraceResults, client comms.TraceClient){
			defer close(out)
			
			// Create a timeout for the trace operation.
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond * time.Duration(timeout))
			defer cancel()
			
			// Attempt to trace.
			results, err := client.BulkTrace(ctx, order)
			if err == nil {
				out <- results
			}else{
				log.Printf("Failed to trace: %v.\n", err)
			}
			
			func() {
				p.mu.Lock()
				defer p.mu.Unlock()
				
				// Complete the task and re-arrange the heap (if the assignee is still in it).
				assignee.tasks -= 1
				if assignee.index < uint(len(p.heap)) && p.heap[assignee.index] == assignee {
					p.bubbleUp(assignee)
				}
				
				// If this is the worker's last task, close the connection.
				if assignee.closing && assignee.tasks == 0 {
					assignee.connection.Close()
				}
			}()
		}(resultsCh, comms.NewTraceClient(assignee.connection))
		
		return resultsCh, nil
	}else{
		return nil, fmt.Errorf("No workers to which task %v can be assigned.", *order)
	}
}

// remove removes a worker with some address from a pool.
// This function assumes that the pool has already been locked.
// This function also assumes that address refers to w, and that w is in the pool.
func (p *Pool) remove(address string, w *worker) {
	wIndex := w.index
	
	// Remove the worker from the pool.
	delete(p.addresses, address)
	p.swap(uint(len(p.heap)) - 1, wIndex)
	p.heap = p.heap[:len(p.heap) - 1]
	
	// If necessary, re-arrange the heap.
	if wIndex < uint(len(p.heap)) {
		p.bubbleDown(p.heap[wIndex])
	}
	
	// Close the worker and disconnect if there are no remaining tasks.
	w.closing = true
	if w.tasks == 0 {
		w.connection.Close()
	}
}

// heartbeat periodically sends out heartbeat messages to a worker.
// This function should be spun off as a goroutine.
func (p *Pool) heartbeat(w *worker) {
	for beat := true; beat; {
		select{
		case <-w.stopHeartbeats:
			beat = false
		case <-time.After(time.Millisecond * time.Duration(HeartbeatFrequency)):
			func() {
				// Because ClientConn objects are threadsafe, we don't need to lock.
				client := comms.NewTraceClient(w.connection)
				
				// Set up a timeout for the heartbeat.
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond * time.Duration(HeartbeatTimeout))
				defer cancel()
				
				// Attempt to send a heartbeat.
				if _, err := client.Heartbeat(ctx, &empty.Empty{}); err != nil {
					log.Printf("Failed to send heartbeat: %v.\n", err)
					
					func() {
						p.mu.Lock()
						defer p.mu.Unlock()
						
						// Find whether the worker is in the pool, then remove it if it is.
						for a, wInternal := range p.addresses {
							if w == wInternal {
								p.remove(a, w)
								break
							}
						}
					}()
					
					beat = false
				}
			}()
		}
	}
}

// Add adds a new worker to the pool.
func (p *Pool) Add(address string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if _, exists := p.addresses[address]; !exists {
		// Connect to the worker.
		// This ClientConn is threadsafe.
		conn, err := grpc.Dial(address, grpc.WithInsecure())
		if err != nil {
			return err
		}
		
		// Set up a new worker.
		w := &worker{connection: conn, stopHeartbeats: make(chan struct{}), closing: false, tasks: 0, index: uint(len(p.heap))}
		
		// Add the worker to the pool.
		p.addresses[address] = w
		p.heap = append(p.heap, w)
		p.bubbleUp(w)
		
		// Spin off a goroutine to send the worker heartbeats.
		go p.heartbeat(w)
	}
	
	return nil
}

// Remove removes a worker from the pool.
func (p *Pool) Remove(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if w, exists := p.addresses[address]; exists {
		// Stop the worker from recieving heartbeats.
		w.stopHeartbeats <- struct{}{}
		
		p.remove(address, w)
	}
}