package main

import (
	"github.com/mwindels/distributed-raytracer/shared/comms"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/worker/shared/tracer"
	"google.golang.org/grpc"
	"encoding/gob"
	"math/rand"
	"context"
	"bytes"
	"time"
	"net"
	"fmt"
	"os"
)

type Tracer struct {
	screenWidth, screenHeight int
	scene state.Environment
}

func (t *Tracer) BulkTrace(ctx context.Context, in *comms.WorkOrder) (*comms.TraceResults, error) {
	results := new(comms.TraceResults)
	
	// If screen dimensions were sent, update the tracer's screen dimensions.
	if in.GetScreenWidth() != 0 {
		t.screenWidth = int(in.GetScreenWidth())
	}
	if in.GetScreenHeight() != 0 {
		t.screenHeight = int(in.GetScreenHeight())
	}
	
	// Update the scene's state.
	// Right now lets just assume that the whole state is sent every time.
	if in.GetState() != nil {
		var newScene state.Environment
		if err := gob.NewDecoder(bytes.NewBuffer(in.GetState())).Decode(newScene); err != nil {
			return nil, err
		}
		
		t.scene = newScene
	}
	
	// For every pixel specified...
	for i := in.GetX(); i < in.GetWidth(); i++ {
		for j := in.GetY(); j < in.GetHeight(); j++ {
			if colour, valid := tracer.Trace(int(i), int(j), t.screenWidth, t.screenHeight, &t.scene); valid {
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

func main() {
	// Seed the rand package.
	rand.Seed(time.Now().UTC().UnixNano())
	
	// Make sure we have enough parameters.
	if len(os.Args) != 2 {
		fmt.Println("Improper parameters.  This program requires the parameters:"+
			"\n\t(1) listening port")
		os.Exit(1)
	}
	
	// Set up and register the worker.
	server := grpc.NewServer()
	comms.RegisterTraceServer(server, new(Tracer))
	
	// Create a listener on the given port.
	listener, err := net.Listen("tcp", ":" + os.Args[1])
	if err != nil {
		panic(err)
	}
	
	// Serve incoming work orders.
	if err = server.Serve(listener); err != nil {
		panic(err)
	}
}