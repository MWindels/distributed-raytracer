// Package input provides functionality for event parsing.
package input

import "github.com/veandco/go-sdl2/sdl"

// These constants are movement direction masks that should be applied to the second return value of HandleInputs.
const (
	MoveForward uint8 = 1 << iota
	MoveLeftward
	MoveBackward
	MoveRightward
	MoveUpward
	MoveDownward
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
			if keyEvent.Type == sdl.KEYDOWN {
				switch keyEvent.Keysym.Sym {
				case sdl.K_ESCAPE:
					running = false
					break
				case sdl.K_w:
					if moveDirs & MoveBackward != 0 {
						moveDirs &^= MoveForward | MoveBackward
					}else{
						moveDirs |= MoveForward
					}
					break
				case sdl.K_a:
					if moveDirs & MoveRightward != 0 {
						moveDirs &^= MoveLeftward | MoveRightward
					}else{
						moveDirs |= MoveLeftward
					}
					break
				case sdl.K_s:
					if moveDirs & MoveForward != 0 {
						moveDirs &^= MoveBackward | MoveForward
					}else{
						moveDirs |= MoveBackward
					}
					break
				case sdl.K_d:
					if moveDirs & MoveLeftward != 0 {
						moveDirs &^= MoveRightward | MoveLeftward
					}else{
						moveDirs |= MoveRightward
					}
					break
				case sdl.K_SPACE:
					if moveDirs & MoveDownward != 0 {
						moveDirs &^= MoveUpward | MoveDownward
					}else{
						moveDirs |= MoveUpward
					}
					break
				case sdl.K_LSHIFT:
					if moveDirs & MoveUpward != 0 {
						moveDirs &^= MoveDownward | MoveUpward
					}else{
						moveDirs |= MoveDownward
					}
					break
				}
			}else if keyEvent.Type == sdl.KEYUP {
				switch keyEvent.Keysym.Sym {
				case sdl.K_w:
					moveDirs &^= MoveForward
					break
				case sdl.K_a:
					moveDirs &^= MoveLeftward
					break
				case sdl.K_s:
					moveDirs &^= MoveBackward
					break
				case sdl.K_d:
					moveDirs &^= MoveRightward
					break
				case sdl.K_SPACE:
					moveDirs &^= MoveUpward
					break
				case sdl.K_LSHIFT:
					moveDirs &^= MoveDownward
					break
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