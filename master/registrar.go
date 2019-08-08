package main

import (
	"github.com/mwindels/distributed-raytracer/shared/comms"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc"
	"encoding/gob"
	"context"
	"strconv"
	"strings"
	"unicode"
	"bytes"
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
		r.sys.mu.RLock()
		defer r.sys.mu.RUnlock()
		
		// Encode the scene state.
		err = encoder.Encode(r.sys.scene)
	}()
	
	// If there was an error while encoding, return it.
	if err != nil {
		return nil, err
	}
	
	// Add the worker to the workers map.
	if err = r.sys.workers.Add(addr); err != nil {
		return nil, err
	}
	
	// Build up the repsonse.
	stateData := comms.MasterState{
		State: writer.Bytes(),
		ScreenWidth: uint32(r.screenWidth),
		ScreenHeight: uint32(r.screenHeight),
	}
	
	return &stateData, nil
}

// newRegistrar sets up a new registration server.
func newRegistrar(sys *system, server *grpc.Server, screenWidth, screenHeight, registrationPort uint) {
	// Set up the registration server.
	comms.RegisterRegistrationServer(server, &Registrar{sys: sys, screenWidth: screenWidth, screenHeight: screenHeight})
	
	// Create a listener for the workers.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", registrationPort))
	if err != nil {
		log.Fatalf("Failed to listen on port \"%d\": %v.\n", registrationPort, err)
	}
	
	// Serve incoming registration orders.
	if err = server.Serve(listener); err != nil {
		log.Fatalf("Registrar interrupted: %v.\n", err)
	}
}