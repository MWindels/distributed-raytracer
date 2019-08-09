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
	"reflect"
	"bytes"
	"sync"
	"math"
	"sort"
	"log"
	"os"
)

// widthKernel and heightKernel both inform the recursion depth of the screen partitioning function.
// If there are sufficient workers, these values represent the largest width and height a minimal partition piece can be.
const (
	widthKernel uint32 = 50
	heightKernel uint32 = 50
)

// workerRedundancy controls how many workers are assigned to each partition of the screen.
const workerRedundancy uint = 2

// traceTimeout controls how long the master waits before rejecting a BulkTrace call.
// This is a variable because the master may want to dynamically change it.
var traceTimeout uint = 1000

// these variables are used to calculate the number of frames per second.
var (
	frameStartTimes []uint32 = nil
	frameEndTimes []uint32 = nil
)

// system represents the whole distributed system as the master sees it.
type system struct {
	mu sync.RWMutex	// Used to protect the scene's state.
	scene state.Environment
	
	workers pool.Pool
}

// partition recursively creates a list of work orders by partitioning an area.
// The first return value is a slice of the original area's partitioned sub-areas.
// The second return value is the number of leftover workers.
func partition(area *comms.WorkOrder, workers uint, dimension uint) ([]comms.WorkOrder, uint) {
	// If there aren't enough workers left to split the area in half, return.
	if workers / workerRedundancy < 2 {
		if workers > workerRedundancy {
			return []comms.WorkOrder{*area}, workers % workerRedundancy
		}else{
			return []comms.WorkOrder{*area}, 0
		}
	}
	
	x, y := area.GetX(), area.GetY()
	width, height := area.GetWidth(), area.GetHeight()
	if width <= widthKernel && height <= heightKernel {
		// If the area can't be partitioned any more, return.
		return []comms.WorkOrder{*area}, workers - workerRedundancy
	}else if width <= widthKernel {
		// If the area can't be split vertically, split horizontally.
		dimension = 1
	}else if height <= heightKernel {
		// If the area can't be split horizontally, split vertically.
		dimension = 0
	}
	
	// Compute the left and right areas.
	var leftOrder, rightOrder *comms.WorkOrder
	if dimension % 2 == 0 {
		leftOrder = &comms.WorkOrder{X: x, Y: y, Width: width / 2, Height: height, Diff: area.GetDiff()}
		rightOrder = &comms.WorkOrder{X: x + width / 2, Y: y, Width: width / 2 + width % 2, Height: height, Diff: area.GetDiff()}
	}else{
		leftOrder = &comms.WorkOrder{X: x, Y: y, Width: width, Height: height / 2, Diff: area.GetDiff()}
		rightOrder = &comms.WorkOrder{X: x, Y: y + height / 2, Width: width, Height: height / 2 + height % 2, Diff: area.GetDiff()}
	}
	
	// Find the partitions within the left and right areas.
	left, remainder := partition(leftOrder, workers / 2 + workers % 2, (dimension + 1) % 2)
	right, remainder := partition(rightOrder, workers / 2 + remainder, (dimension + 1) % 2)
	return append(left, right...), remainder
}

// newCoordinator coordinates the drawing of a new frame.
func newCoordinator(sys *system, diff []byte, frame uint, window *sdl.Window, surface *sdl.Surface, in <-chan struct{}, out chan<- struct{}) {
	// Find the number of workers.
	// This number might change while assigning tasks, so this is just a heuristic for partitioning.
	numWorkers := sys.workers.Size()
	
	if numWorkers > 0 {
		// Partition the screen.
		partitions, _ := partition(&comms.WorkOrder{X: 0, Y: 0, Width: uint32(surface.W), Height: uint32(surface.H), Diff: diff}, numWorkers, 0)
		
		// Assign the partitions to workers.
		resultMap := make(map[<-chan *comms.TraceResults]*comms.WorkOrder)
		resultChs := make([]reflect.SelectCase, 0, workerRedundancy * uint(len(partitions)))
		for _, p := range partitions {
			var err error
			assigned := false
			
			// Assign worker(s) to the current partition.
			for i := uint(0); i < workerRedundancy; i++ {
				if resultCh, err := sys.workers.Assign(&p, traceTimeout); err == nil {
					resultMap[resultCh] = &p
					resultChs = append(resultChs, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(resultCh)})
					assigned = true
				}
			}
			
			// If no workers could be assigned to this partition, skip the frame.
			if !assigned {
				<-in
				log.Printf("Frame %d skipped, could not draw part of screen: %v.\n", frame, err)
				out <- struct{}{}
				return
			}
		}
		
		// Accumulate results.
		orderMap := make(map[*comms.WorkOrder]*comms.TraceResults)
		for len(orderMap) < len(partitions) {
			// Wait for a worker to respond.
			idx, value, success := reflect.Select(resultChs)
			result := value.Interface().(*comms.TraceResults)
			order := resultMap[resultChs[idx].Chan.Interface().(<-chan *comms.TraceResults)]
			
			// Update the order map with the new results.
			if status, exists := orderMap[order]; exists {
				if success && status == nil {
					orderMap[order] = result
				}
			}else{
				if success {
					orderMap[order] = result
				}else{
					orderMap[order] = nil
				}
			}
			
			// Remove the worker from the working list.
			resultChs = append(resultChs[:idx], resultChs[idx + 1:]...)
		}
		
		// If any of the partitions could not be filled, skip the frame.
		for _, r := range orderMap {
			if r == nil {
				<-in
				log.Printf("Frame %d skipped, could not draw part of the screen.", frame)
				out <- struct{}{}
				return
			}
		}
		
		// Draw the frame.
		<-in
		frameEndTimes = append(frameEndTimes, sdl.GetTicks())
		surface.FillRect(nil, 0)
		for o, r := range orderMap {
			pixels := r.GetResults()
			xFirst, xLast := int(o.GetX()), int(o.GetX() + o.GetWidth())
			yFirst, yLast := int(o.GetY()), int(o.GetY() + o.GetHeight())
			for i := xFirst; i < xLast; i++ {
				for j := yFirst; j < yLast; j++ {
					pixel := pixels[i * int(surface.H) + j]
					surface.Set(i, j, colour.NewRGB(uint8(pixel.GetR()), uint8(pixel.GetG()), uint8(pixel.GetB())))
				}
			}
		}
		window.UpdateSurface()
		frameStartTimes = append(frameStartTimes, sdl.GetTicks())
		out <- struct{}{}
	}else{
		// If there are no workers available, skip the frame.
		<-in
		log.Printf("Frame %d skipped, no workers in pool.\n", frame)
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
	go newRegistrar(&sys, registrar, uint(surface.W), uint(surface.H), uint(registrationPort))
	
	// Get the initial coordinator channel ready.
	coordinatorIn := make(chan struct{}, 1)
	coordinatorIn <- struct{}{}
	
	// Parse user input and issue work orders.
	var frame uint = 0
	var prevUpdate, currentUpdate uint32
	for running, moveDirs, yaw, pitch := true, uint8(0), 0.0, 0.0; running; {
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
	
	// Log the total number of frames and some FPS stats.
	log.Printf("Total frames drawn: %d.\n", len(frameEndTimes))
	log.Printf("Total frames: %d.\n", frame)
	usableFrames := len(frameEndTimes) - 1
	if usableFrames > 0 {
		frameEndTimes = frameEndTimes[1:]
		frameStartTimes = frameStartTimes[:len(frameStartTimes) - 1]
		
		// Compute the frames per second for each frame.
		durationSum := uint32(0)
		var fpsPerFrame sort.Float64Slice = make([]float64, usableFrames, usableFrames)
		for i := 0; i < usableFrames; i++ {
			durationSum += (frameEndTimes[i] - frameStartTimes[i])
			fpsPerFrame[i] = float64(i + 1) / math.Max(float64(durationSum) / 1000.0, 0.001)
		}
		fpsPerFrame.Sort()
		
		// Compute the mean FPS value.
		fpsMean := 0.0
		for _, fps := range fpsPerFrame {
			fpsMean += fps
		}
		fpsMean /= float64(usableFrames)
		
		// Compute the FPS values' standard deviation.
		fpsStdDev := 0.0
		for _, fps := range fpsPerFrame {
			dev := fps - fpsMean
			fpsStdDev += dev * dev
		}
		fpsStdDev = math.Sqrt(fpsStdDev / float64(usableFrames))
		
		// Log stats.
		log.Printf("Mean FPS: %f.\n", fpsMean)
		log.Printf("Median FPS: %f.\n", fpsPerFrame[usableFrames / 2])
		log.Printf("FPS Standard Deviation: %f.\n", fpsStdDev)
		log.Printf("FPS Range: [%f, %f].\n", fpsPerFrame[0], fpsPerFrame[len(fpsPerFrame) - 1])
	}
}