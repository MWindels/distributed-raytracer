package main

import (
	//"github.com/mwindels/distributed-raytracer/shared/comms"
	"github.com/mwindels/distributed-raytracer/shared/state"
	//"github.com/mwindels/distributed-raytracer/shared/screen"
	//"google.golang.org/grpc"
	//"context"
	"strconv"
	"sync"
	//"net"
	"log"
	"os"
)

// maxUpdates controls how many frames state updates are kept for.
const maxUpdates uint = 500

// traceTimeout controls how long the master waits before rejecting a BulkTrace call.
// This is a variable because the master may want to dynamically change it.
var traceTimeout uint = 300

// workerStats stores worker-related information.
type workerStats struct {
	lastFrameCompleted uint
	lastFrameStarted uint
}

// system represents the whole distributed system as the master sees it.
type system struct {
	mu sync.Mutex
	
	frame uint
	scene state.Environment
	updates []interface{}
	
	workers map[string]workerStats
}

func main() {
	// Make sure we have enough parameters.
	if len(os.Args) != 5 {
		log.Fatalln("Improper parameters.  This program requires the parameters:"+
			"\n\t(1) environment file path"+
			"\n\t(2) window width"+
			"\n\t(3) window height"+
			"\n\t(4) worker registration port")
	}
	
	// Parse the command line parameters.
	env, err := state.EnvironmentFromFile(os.Args[1])
	if err != nil {
		log.Fatalf("Could not read in environment \"%s\": %v.\n", os.Args[1], err)
	}
	width, err := strconv.ParseUint(os.Args[2], 10, 64)
	if err != nil {
		log.Fatalf("Could not parse window width \"%s\": %v.\n", os.Args[2], err)
	}
	height, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		log.Fatalf("Could not parse window height \"%s\": %v.\n", os.Args[3], err)
	}
	registrationPort, err := strconv.ParseUint(os.Args[4], 10, 32)
	if err != nil {
		log.Fatalf("Could not parse port number \"%s\": %v.\n", os.Args[4], err)
	}
	
	// Set up the system's state.
	sys := system{frame: 0, scene: env, updates: make([]interface{}, 0, maxUpdates), workers: make(map[string]workerStats)}
	
	// Set up the screen.
	/*window, surface, err := screen.StartScreen("Distributed Ray-Tracer", int(width), int(height))
	if err != nil {
		log.Fatalf("Could not start screen: %v.\n", err)
	}
	defer screen.StopScreen(window)*/
	
	// Spin off the registration server.
	/*go*/ newRegistrar(&sys, uint(width), uint(height), uint(registrationPort))
	
}