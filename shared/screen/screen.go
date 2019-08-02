// Package screen provides screen-related functionality for use by the master or a sequential worker.
package screen

import (
	"github.com/veandco/go-sdl2/sdl"
	"fmt"
)

// These constants are timing values related to screen-updating.
const (
	FPS uint32 = 30
	MsPerFrame uint32 = 1000 / FPS
)

// StartScreen initializes SDL2 and a new window.
func StartScreen(name string, width, height int) (*sdl.Window, *sdl.Surface, error) {
	complete := false
	
	// Start SDL2.
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return nil, nil, err
	}
	defer func() {
		if !complete {
			sdl.Quit()	// Only want to call Quit if this function doesn't complete.
		}
	}()
	
	// Create new window.
	window, err := sdl.CreateWindow(name, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, int32(width), int32(height), sdl.WINDOW_SHOWN)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if !complete {
			window.Destroy()	// Again, only want to call if this function doesn't complete.
		}
	}()
	
	// Get the screen from the new window.
	surface, err := window.GetSurface()
	if err != nil {
		return nil, nil, err
	}
	
	// Set mouse mode to relative.
	if sdl.SetRelativeMouseMode(true) != 0 {
		return nil, nil, fmt.Errorf("Relative mouse mode is not supported.")
	}
	
	complete = true
	return window, surface, nil
}

// StopScreen closes SDL2 and some window.
func StopScreen(window *sdl.Window) {
	window.Destroy()
	sdl.Quit()
}