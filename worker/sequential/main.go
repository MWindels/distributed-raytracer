package main

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/shared/screen"
	"github.com/mwindels/distributed-raytracer/shared/input"
	"github.com/mwindels/distributed-raytracer/worker/shared/tracer"
	"image/color"
	"math/rand"
	"strconv"
	"time"
	"fmt"
	"os"
)

// draw draws an environment to the screen.
func draw(window *sdl.Window, surface *sdl.Surface, env *state.Environment) {
	// Clear the screen.
	surface.FillRect(nil, 0)
	
	// For every pixel on screen...
	width, height := int(surface.W), int(surface.H)
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			// If an object was hit, colour a pixel.
			if colour, valid := tracer.Trace(i, j, width, height, env); valid {
				r, g, b := colour.RGB()
				surface.Set(i, j, color.RGBA{R: r, G: g, B: b, A: 0x00})
			}
		}
	}
	
	//Update the screen.
	window.UpdateSurface()
}

func main() {
	// Seed the rand package.
	rand.Seed(time.Now().UTC().UnixNano())
	
	// Make sure we have enough parameters.
	if len(os.Args) != 4 {
		fmt.Println("Improper parameters.  This program requires the parameters:"+
			"\n\t(1) environment file path"+
			"\n\t(2) window width"+
			"\n\t(3) window height")
		os.Exit(1)
	}
	
	// Load in the environment.
	env, err := state.EnvironmentFromFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	
	// Get the width and height of the screen.
	width, err := strconv.ParseUint(os.Args[2], 10, 64)
	if err != nil {
		panic(err)
	}
	height, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		panic(err)
	}
	
	// Start the screen.
	window, surface := screen.StartScreen("Sequential Ray-Tracer", int(width), int(height))
	defer screen.StopScreen(window)
	
	// Run the input/update/render loop.
	var prevUpdate, currentUpdate uint32
	for running, moveDirs, yaw, pitch := true, uint8(0), 0.0, 0.0; running; {
		prevUpdate = sdl.GetTicks()
		
		// Handle new inputs.
		running, moveDirs, yaw, pitch = input.HandleInputs(moveDirs, int(surface.W), int(surface.H))
		
		// If the camera needs to move, move it.
		env.Cam.Move(0.1, moveDirs & input.MoveForward != 0, moveDirs & input.MoveBackward != 0, moveDirs & input.MoveLeftward != 0, moveDirs & input.MoveRightward != 0, moveDirs & input.MoveUpward != 0, moveDirs & input.MoveDownward != 0)
		
		// If the camera needs to rotate, rotate it.
		env.Cam.Yaw(yaw * env.Cam.Fov / 2.0)
		env.Cam.Pitch(pitch * (float64(surface.H) / float64(surface.W)) * env.Cam.Fov / 2.0)
		
		// Draw the screen.
		draw(window, surface, &env)
		
		// If there's still time before the next frame needs to be drawn, wait.
		currentUpdate = sdl.GetTicks()
		if currentUpdate - prevUpdate < screen.MsPerFrame {
			sdl.Delay(screen.MsPerFrame - (currentUpdate - prevUpdate))
		}
	}
}