package main

import (
	"github.com/mwindels/distributed-raytracer/shared/comms"
	"github.com/mwindels/distributed-raytracer/shared/screen"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc"
	"encoding/gob"
	"context"
	"strconv"
	"strings"
	"unicode"
	"bytes"
	"time"
	"net"
	"log"
	"fmt"
)

// Registrar implements the comms.RegistrationServer interface.
type Registrar struct {
	sys *system
	screenWidth, screenHeight uint
}

// Register registers a worker with the master.
func (r *Registrar) Register(ctx context.Context, req *comms.WorkerLink) (*comms.MasterState, error) {
	var err error = nil
	var firstFrame uint
	
	// Get a writer and encoder ready for processing state.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Get the worker's sending address.
	worker, exists := peer.FromContext(ctx)
	if !exists {
		return nil, fmt.Errorf("Could not derive worker's address.")
	}
	
	// Compute the worker's recieving address.
	addr := strings.Join([]string{strings.TrimRightFunc(worker.Addr.String(), unicode.IsNumber), strconv.FormatUint(uint64(req.GetPort()), 10)}, "")
	
	func() {
		r.sys.mu.Lock()
		defer r.sys.mu.Unlock()
		
		// Add the worker to the workers map.
		firstFrame = r.sys.frame
		r.sys.workers[addr] = workerStats{lastFrameCompleted: firstFrame, lastFrameStarted: firstFrame}
		
		// Encode the scene state.
		err = encoder.Encode(r.sys.scene)
	}()
	
	// If there was an error while encoding, return it.
	if err != nil {
		return nil, err
	}
	
	// Spin off a thread that'll automatically remove the worker if they don't do enough work.
	go func() {
		for working := true; working; {
			// Wait for a duration (this is a heuristic).
			time.Sleep(time.Millisecond * time.Duration(maxUpdates * uint(screen.MsPerFrame)))
			
			func() {
				r.sys.mu.Lock()
				defer r.sys.mu.Unlock()
				
				if stats, exists := r.sys.workers[addr]; exists {
					// If the worker has fallen too far behind, assume it has failed.
					if r.sys.frame - stats.lastFrameStarted > uint(len(r.sys.updates)) {
						// TODO: gracefully close the trace connection.
						delete(r.sys.workers, addr)
						working = false
					}
				}else{
					working = false
				}
			}()
		}
	}()
	
	// Build up the repsonse.
	stateData := comms.MasterState{
		State: writer.Bytes(),
		Frame: uint32(firstFrame),
		ScreenWidth: uint32(r.screenWidth),
		ScreenHeight: uint32(r.screenHeight),
	}
	
	return &stateData, nil
}

// newRegistrar sets up a new registration server.
func newRegistrar(sys *system, screenWidth, screenHeight, registrationPort uint) {
	// Set up the registration server.
	server := grpc.NewServer()
	comms.RegisterRegistrationServer(server, &Registrar{sys: sys, screenWidth: screenWidth, screenHeight: screenHeight})
	
	// Create a listener for the workers.
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", registrationPort))
	if err != nil {
		log.Fatalf("Failed to listen on port \"%d\": %v.\n", registrationPort, err)
	}
	
	// Serve incoming registration orders.
	if err = server.Serve(listener); err != nil {
		log.Fatalf("Connection interrupted: %v.\n", err)
	}
}