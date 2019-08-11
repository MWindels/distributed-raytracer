package main

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/shared/screen"
	"github.com/mwindels/distributed-raytracer/shared/input"
	"github.com/mwindels/distributed-raytracer/worker/shared/tracer"
	"strconv"
	"log"
	"os"
)

// draw draws an environment to the screen.
func draw(window *sdl.Window, surface *sdl.Surface, env *state.EnvMutables) {
	// Clear the screen.
	surface.FillRect(nil, 0)
	
	// For every pixel on screen...
	width, height := int(surface.W), int(surface.H)
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			// If an object was hit, colour a pixel.
			if colour, valid := tracer.Trace(i, j, width, height, env); valid {
				surface.Set(i, j, colour)
			}
		}
	}
	
	//Update the screen.
	window.UpdateSurface()
}

func main() {
	// Make sure we have enough parameters.
	if len(os.Args) != 4 {
		log.Fatalln("Improper parameters.  This program requires the parameters:"+
			"\n\t(1) environment file path"+
			"\n\t(2) window width"+
			"\n\t(3) window height")
	}
	
	// Load in the environment.
	env, err := state.EnvironmentFromFile(os.Args[1])
	if err != nil {
		log.Fatalf("Could not read in environment \"%s\": %v.\n", os.Args[1], err)
	}
	
	// Get the width and height of the screen.
	width, err := strconv.ParseUint(os.Args[2], 10, 64)
	if err != nil {
		log.Fatalf("Could not parse window width \"%s\": %v.\n", os.Args[2], err)
	}
	height, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		log.Fatalf("Could not parse window height \"%s\": %v.\n", os.Args[3], err)
	}
	
	// Start the screen.
	window, surface, err := screen.StartScreen("Sequential Ray-Tracer", int(width), int(height))
	if err != nil {
		log.Fatalf("Could not start screen: %v.\n", err)
	}
	defer screen.StopScreen(window)
	
	// Run the input/update/render loop.
	scene := env.Mutable()
	/*firstUpdate := sdl.GetTicks()*/
	var prevUpdate, currentUpdate uint32
	for running, /*frame,*/ moveDirs, yaw, pitch := true, /*uint(0),*/ uint8(0), 0.0, 0.0; running; /*frame++*/ {
		prevUpdate = sdl.GetTicks()
		
		// Handle new inputs.
		running, moveDirs, yaw, pitch = input.HandleInputs(moveDirs, int(surface.W), int(surface.H))
		
		// If the camera needs to move, move it.
		scene.Cam.Move(0.1, moveDirs & input.MoveForward != 0, moveDirs & input.MoveBackward != 0, moveDirs & input.MoveLeftward != 0, moveDirs & input.MoveRightward != 0, moveDirs & input.MoveUpward != 0, moveDirs & input.MoveDownward != 0)
		
		// If the camera needs to rotate, rotate it.
		scene.Cam.Yaw(yaw * scene.Cam.Fov / 2.0)
		scene.Cam.Pitch(pitch * (float64(surface.H) / float64(surface.W)) * scene.Cam.Fov / 2.0)
		
		// Draw the screen.
		draw(window, surface, scene)
		
		// If there's still time before the next frame needs to be drawn, wait.
		currentUpdate = sdl.GetTicks()
		/*log.Printf("\t%f\n", float64(frame) / (float64(currentUpdate - firstUpdate) / 1000.0))*/
		if currentUpdate - prevUpdate < screen.MsPerFrame {
			sdl.Delay(screen.MsPerFrame - (currentUpdate - prevUpdate))
		}
	}
}