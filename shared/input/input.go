// Package input provides functionality for event parsing.
package input

import "github.com/veandco/go-sdl2/sdl"

// These constants are movement direction masks that should be applied to the second return value of HandleInputs.
const (
	MoveForward uint8 = 1 << iota
	MoveLeftward
	MoveBackward
	MoveRightward
)

// HandleInputs parses all input events waiting in the queue.
// This function returns: (running, new move directions, yaw, pitch).
func HandleInputs(moveDirs uint8, width, height int) (bool, uint8, float64, float64) {
	running := true	// We assume this to be true.
	yaw, pitch := 0.0, 0.0	// These are measured in units of (fov / 2) radians.
	
	// Pull every event out of the queue and evaluate/apply it.
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch event.(type) {
		case *sdl.KeyboardEvent:
			keyEvent := event.(*sdl.KeyboardEvent)
			if keyEvent.Keysym.Mod == sdl.KMOD_NONE {
				if keyEvent.Type == sdl.KEYDOWN {
					switch keyEvent.Keysym.Sym {
					case sdl.K_ESCAPE:
						running = false
						break
					case sdl.K_UP:
						if moveDirs & MoveBackward != 0 {
							moveDirs &^= MoveForward | MoveBackward
						}else{
							moveDirs |= MoveForward
						}
						break
					case sdl.K_LEFT:
						if moveDirs & MoveRightward != 0 {
							moveDirs &^= MoveLeftward | MoveRightward
						}else{
							moveDirs |= MoveLeftward
						}
						break
					case sdl.K_DOWN:
						if moveDirs & MoveForward != 0 {
							moveDirs &^= MoveBackward | MoveForward
						}else{
							moveDirs |= MoveBackward
						}
						break
					case sdl.K_RIGHT:
						if moveDirs & MoveLeftward != 0 {
							moveDirs &^= MoveRightward | MoveLeftward
						}else{
							moveDirs |= MoveRightward
						}
						break
					}
				}else if keyEvent.Type == sdl.KEYUP {
					switch keyEvent.Keysym.Sym {
					case sdl.K_UP:
						moveDirs &^= MoveForward
						break
					case sdl.K_LEFT:
						moveDirs &^= MoveLeftward
						break
					case sdl.K_DOWN:
						moveDirs &^= MoveBackward
						break
					case sdl.K_RIGHT:
						moveDirs &^= MoveRightward
						break
					}
				}
			}
			break
		case *sdl.MouseMotionEvent:
			mouseEvent := event.(*sdl.MouseMotionEvent)
			yaw += float64(mouseEvent.XRel) / float64(width / 2)
			pitch -= float64(mouseEvent.YRel) / float64(height / 2)
			break
		}
	}
	return running, moveDirs, yaw, pitch
}