package main

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/mwindels/distributed-raytracer/shared/comms"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/shared/screen"
	"github.com/mwindels/distributed-raytracer/shared/input"
	"github.com/mwindels/distributed-raytracer/master/pool"
	"google.golang.org/grpc"
	"encoding/gob"
	"strconv"
	"bytes"
	"sync"
	"log"
	"os"
)

// traceTimeout controls how long the master waits before rejecting a BulkTrace call.
// This is a variable because the master may want to dynamically change it.
var traceTimeout uint = 10000

// system represents the whole distributed system as the master sees it.
type system struct {
	mu sync.RWMutex	// Used to protect the scene's state.
	scene state.Environment
	
	workers pool.Pool
}

// newCoordinator coordinates the drawing of a new frame.
func newCoordinator(sys *system, diff []byte, frame uint, window *sdl.Window, surface *sdl.Surface, in <-chan struct{}, out chan<- struct{}) {
	// Find the number of workers.
	// This number might change while assigning tasks, so this is just a heuristic for partitioning.
	numWorkers := sys.workers.Size()
	
	if numWorkers > 0 {
		// TEMPORARY!
		resultCh, err := sys.workers.Assign(&comms.WorkOrder{X: 0, Y: 0, Width: uint32(surface.W), Height: uint32(surface.H), Diff: diff}, traceTimeout)
		if err == nil {
			results := (<-resultCh).GetResults()
			
			<-in
			if results != nil {
				surface.FillRect(nil, 0)
				for i := 0; i < int(surface.W); i++ {
					for j := 0; j < int(surface.H); j++ {
						pixel := results[i * int(surface.H) + j]
						surface.Set(i, j, colour.NewRGB(uint8(pixel.GetR()), uint8(pixel.GetG()), uint8(pixel.GetB())))
					}
				}
				window.UpdateSurface()
			}
			out <- struct{}{}
		}else{
			out <- <-in
		}
		// TEMPORARY!
	}else{
		// If there are no workers available, skip the frame.
		<-in
		log.Printf("No workers in pool, frame %d skipped.\n", frame)
		out <- struct{}{}
	}
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
	sys := system{scene: env, workers: pool.NewPool(8)}
	defer sys.workers.Destroy()
	
	// Set up the screen.
	window, surface, err := screen.StartScreen("Distributed Ray-Tracer", int(width), int(height))
	if err != nil {
		log.Fatalf("Could not start screen: %v.\n", err)
	}
	defer screen.StopScreen(window)
	
	// Spin off the registration server.
	registrar := grpc.NewServer()
	defer registrar.GracefulStop()
	go newRegistrar(&sys, registrar, uint(width), uint(height), uint(registrationPort))
	
	// Get the initial coordinator channel ready.
	coordinatorIn := make(chan struct{}, 1)
	coordinatorIn <- struct{}{}
	
	// Parse user input and issue work orders.
	var prevUpdate, currentUpdate uint32
	for running, frame, moveDirs, yaw, pitch := true, uint(0), uint8(0), 0.0, 0.0; running; {
		prevUpdate = sdl.GetTicks()
		
		// Collect new inputs.
		running, moveDirs, yaw, pitch = input.HandleInputs(moveDirs, int(surface.W), int(surface.H))
		
		if moveDirs != 0 || yaw != 0.0 || pitch != 0.0 {
			func() {
				sys.mu.Lock()
				defer sys.mu.Unlock()
				
				scene := sys.scene.Mutable()
				
				// Move the camera.
				scene.Cam.Move(0.1, moveDirs & input.MoveForward != 0, moveDirs & input.MoveBackward != 0, moveDirs & input.MoveLeftward != 0, moveDirs & input.MoveRightward != 0, moveDirs & input.MoveUpward != 0, moveDirs & input.MoveDownward != 0)
				
				// Rotate the camera.
				scene.Cam.Yaw(yaw * scene.Cam.Fov / 2.0)
				scene.Cam.Pitch(pitch * (float64(surface.H) / float64(surface.W)) * scene.Cam.Fov / 2.0)
				
				// Encode the current state of the scene.
				writer := bytes.Buffer{}
				if err := gob.NewEncoder(&writer).Encode(scene); err == nil {
					// Spin off a coordinator for the new frame.
					coordinatorOut := make(chan struct{}, 1)
					go newCoordinator(&sys, writer.Bytes(), frame, window, surface, coordinatorIn, coordinatorOut)
					coordinatorIn = coordinatorOut
				}else{
					log.Printf("Could not encode frame %d's scene: %v.\n", frame, err)
				}
			}()
			
			frame += 1
		}
		
		// Wait for the next frame.
		currentUpdate = sdl.GetTicks()
		if currentUpdate - prevUpdate < screen.MsPerFrame {
			sdl.Delay(screen.MsPerFrame - (currentUpdate - prevUpdate))
		}
	}
	
	// Wait for the remaining coordinators to complete.
	<- coordinatorIn
}