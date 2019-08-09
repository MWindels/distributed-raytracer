package main

import (
	"github.com/mwindels/distributed-raytracer/shared/comms"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/worker/shared/tracer"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"encoding/gob"
	"context"
	"strconv"
	"bytes"
	"time"
	"net"
	"fmt"
	"log"
	"os"
)

// registerFrequency controls the minimum amount of time this worker will wait before trying to re-register itself after a failure.
const registerFrequency uint = 500

// traceTimeout controls how long this worker will wait for trace requests and heartbeats before closing its trace server.
const traceTimeout uint = 2000

// Tracer implements the comms.TraceServer interface.
type Tracer struct {
	// No lock here because we never mutate this data.
	scene state.Environment
	screenWidth, screenHeight uint
	resetTraceTimeout chan struct{}
}

// timeoutReset resets a tracer's trace timeout.
func (t *Tracer) timeoutReset() {
	defer func() {
		recover()
	}()
	
	// Try to reset the trace timeout.
	// If the channel is closed, this will panic and return immediately.
	t.resetTraceTimeout <- struct{}{}
}

// BulkTrace traces a batch of rays.
func (t *Tracer) BulkTrace(ctx context.Context, req *comms.WorkOrder) (*comms.TraceResults, error) {
	t.timeoutReset()
	
	// Set up this call's results.
	xInit, yInit := int(req.GetX()), int(req.GetY())
	width, height := int(req.GetWidth()), int(req.GetHeight())
	results := &comms.TraceResults{
		Results: make([]*comms.TraceResults_Colour, width * height, width * height),
	}
	
	// Decode the mutable state for this frame.
	var diff state.EnvMutables
	if req.GetDiff() != nil {
		if err := gob.NewDecoder(bytes.NewBuffer(req.GetDiff())).Decode(&diff); err != nil {
			return nil, err
		}
		
		diff.LinkTo(t.scene)
	}
	
	// For every pixel specified...
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			// Set up a default colour.
			var r, g, b uint8 = 0, 0, 0
			
			// Make sure the RPC hasn't been cancelled.
			if err := ctx.Err(); err == context.Canceled {
				return nil, err
			}
			
			// If an object was hit, use its colour.
			if objectColour, valid := tracer.Trace(xInit + i, yInit + j, int(t.screenWidth), int(t.screenHeight), &diff); valid {
				r, g, b = objectColour.RGB()
			}
			
			results.Results[i * height + j] = &comms.TraceResults_Colour{
				R: uint32(r),
				G: uint32(g),
				B: uint32(b),
			}
		}
	}
	
	return results, nil
}

// Heartbeat keeps the worker from disconnecting from the master.
func (t *Tracer) Heartbeat(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	t.timeoutReset()
	
	return &empty.Empty{}, nil
}

// register registers this worker with the master at registerAddr for later communication on listenPort using the tracer it returns.
func register(registerAddr string, listenPort uint32) (Tracer, error) {
	// Connect to the master.
	conn, err := grpc.Dial(registerAddr, grpc.WithInsecure())
	if err != nil {
		return Tracer{}, err
	}
	defer conn.Close()
	
	// Create a registration client.
	client := comms.NewRegistrationClient(conn)
	
	// Attempt to register.
	stateMsg, err := client.Register(context.Background(), &comms.WorkerLink{Port: listenPort})
	if err != nil {
		return Tracer{}, err
	}
	
	// Decode the scene's state.
	var newScene state.Environment
	if stateMsg.GetState() != nil {
		if err = gob.NewDecoder(bytes.NewBuffer(stateMsg.GetState())).Decode(&newScene); err != nil {
			return Tracer{}, err
		}
	}else{
		return Tracer{}, fmt.Errorf("No scene data recieved.")
	}
	
	return Tracer{scene: newScene, screenWidth: uint(stateMsg.GetScreenWidth()), screenHeight: uint(stateMsg.GetScreenHeight()), resetTraceTimeout: make(chan struct{})}, nil
}

func main() {
	// Make sure we have enough parameters.
	if len(os.Args) != 3 {
		log.Fatalln("Improper parameters.  This program requires the parameters:"+
			"\n\t(1) master address (including port)"+
			"\n\t(2) work order listening port")
	}
	
	// Parse the command line parameters.
	masterAddr := os.Args[1]
	orderPort, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil {
		log.Fatalf("Could not parse port number \"%s\": %v.\n", os.Args[2], err)
	}
	
	for {
		// Try to register.
		tracer, err := register(masterAddr, uint32(orderPort))
		if err == nil {
			// Set up the worker.
			server := grpc.NewServer()
			comms.RegisterTraceServer(server, &tracer)
			
			// Create a listener for the master.
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", orderPort))
			if err != nil {
				log.Fatalf("Failed to listen on port \"%d\": %v.\n", orderPort, err)
			}
			
			// Spin off a goroutine which closes the trace server if no requests come in within a timeout.
			go func() {
				for {
					select{
					case <-tracer.resetTraceTimeout:
					case <-time.After(time.Millisecond * time.Duration(traceTimeout)):
						close(tracer.resetTraceTimeout)
						server.GracefulStop()
						return
					}
				}
			}()
			
			// Serve incoming work orders.
			if err = server.Serve(listener); err != nil {
				log.Printf("Tracer interrupted: %v.\n", err)
			}else{
				log.Printf("Tracer timed out after recieving no orders or heartbeats.\n")
			}
		}else{
			log.Printf("Failed to register: %v.\n", err)
		}
		
		// Wait before trying to register again.
		time.Sleep(time.Millisecond * time.Duration(registerFrequency))
	}
}