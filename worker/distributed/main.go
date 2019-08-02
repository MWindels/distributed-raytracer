package main

import (
	"github.com/mwindels/distributed-raytracer/shared/comms"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/worker/shared/tracer"
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

// registrationTimeout controls how long to wait for a registration reply.
const registrationTimeout uint = 10000

// Tracer implements the comms.TraceServer interface.
type Tracer struct {
	frame uint
	scene state.Environment
	screenWidth, screenHeight uint
}

// BulkTrace traces a batch of rays.
func (t *Tracer) BulkTrace(ctx context.Context, req *comms.WorkOrder) (*comms.TraceResults, error) {
	results := new(comms.TraceResults)
	
	// Update the scene's state.
	// TODO...
	
	// Update the frame.
	t.frame = uint(req.GetFrame())
	
	// For every pixel specified...
	for i := req.GetX(); i < req.GetWidth(); i++ {
		for j := req.GetY(); j < req.GetHeight(); j++ {
			if colour, valid := tracer.Trace(int(i), int(j), int(t.screenWidth), int(t.screenHeight), &t.scene); valid {
				// If an object was hit, return its colour.
				r, g, b := colour.RGB()
				resultColour := comms.TraceResults_Colour{R: uint32(r), G: uint32(g), B: uint32(b)}
				results.Results = append(results.Results, &resultColour)
			}else{
				// If nothing was hit, return nothing.
				results.Results = append(results.Results, nil)
			}
		}
	}
	
	return results, nil
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
	
	// Create a timeout for the register operation.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond * time.Duration(registrationTimeout))
	defer cancel()
	
	// Attempt to register.
	stateMsg, err := client.Register(ctx, &comms.WorkerLink{Port: listenPort})
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
	
	return Tracer{frame: uint(stateMsg.GetFrame()), scene: newScene, screenWidth: uint(stateMsg.GetScreenWidth()), screenHeight: uint(stateMsg.GetScreenHeight())}, nil
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
			listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", orderPort))
			if err != nil {
				log.Fatalf("Failed to listen on port \"%d\": %v.\n", orderPort, err)
			}
			
			// Serve incoming work orders.
			if err = server.Serve(listener); err != nil {
				log.Fatalf("Connection interrupted: %v.\n", err)
			}else{
				log.Printf("Connection closed.\n")
			}
		}else{
			log.Printf("Failed to register: %v.\n", err)
		}
	}
}