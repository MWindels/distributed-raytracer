package main

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/shared/screen"
	"github.com/mwindels/distributed-raytracer/shared/input"
	"image/color"
	"math/rand"
	"time"
	"math"
)

// draw draws an environment to the screen.
func draw(window *sdl.Window, surface *sdl.Surface, env state.Environment) {
	// Clear the screen.
	surface.FillRect(nil, 0)
	
	// For every pixel on screen...
	for i := 0; i < int(surface.W); i++ {
		for j := 0; j < int(surface.H); j++ {
			// Determine where the pixel is on the projection plane.
			rayPos := screen.PixelToPoint(i, j, int(surface.W), int(surface.H), env.Cam)
			
			// Search for intersections with every object.
			// We can probably do better than a linear search with octrees or k-d trees.
			nearestDistance := math.Inf(1)
			var nearestObj *geom.Triangle = nil
			for k := 0; k < len(env.Objs); k++ {
				if intersect, hit := env.Objs[k].Intersection(env.Cam.Pos, rayPos.Sub(env.Cam.Pos)); hit {
					intersectDistance := intersect.Sub(env.Cam.Pos).Len()
					if nearestObj == nil || intersectDistance < nearestDistance {
						nearestDistance = intersectDistance
						nearestObj = &env.Objs[k]
					}
				}
			}
			
			// If an object was hit, colour a pixel.
			if nearestObj != nil {
				surface.Set(i, j, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x00})
			}
		}
	}
	
	//Update the screen.
	window.UpdateSurface()
}

func main() {
	// Seed the rand package.
	rand.Seed(time.Now().UTC().UnixNano())
	
	// Start the screen.
	window, surface := screen.StartScreen("Sequential Ray-Tracer", 960, 540)
	defer screen.StopScreen(window)
	
	// Create an environment (should be able to load this from a JSON file or something).
	env := state.Environment{
		Objs: []geom.Triangle{
			//geom.Triangle{geom.Vector{0, 0, 1}, geom.Vector{2, 0, 3}, geom.Vector{1, 2, 2}},
			geom.Triangle{geom.Vector{0, 0, 0}, geom.Vector{2, 0, 0}, geom.Vector{0, 2, 0}},
			//geom.Triangle{geom.Vector{-0.5, 0.25, 0.25}, geom.Vector{-2, 0, -0.75}, geom.Vector{-1, 1.75, 0}},
			//geom.Triangle{geom.Vector{0, 0, 0}, geom.Vector{-2, 0, -2}, geom.Vector{0, 2, 0}},
		},
		Lights: []state.Light{
			state.Light{Pos: geom.Vector{0, 0, 0}, Col: state.RGB{0xFF, 0xFF, 0xFF}},
		},
		Cam: state.NewCamera(geom.Vector{1, 1, -2}, geom.Vector{0, 0, 1}, math.Pi / 3.0),
	}
	
	// Run the input/update/render loop.
	var prevUpdate, currentUpdate uint32
	for running, moveDirs, yaw, pitch := true, uint8(0), 0.0, 0.0; running; {
		prevUpdate = sdl.GetTicks()
		
		// Handle new inputs.
		running, moveDirs, yaw, pitch = input.HandleInputs(moveDirs, int(surface.W), int(surface.H))
		
		// Check whether the camera needs to move.
		moveVector := geom.Vector{0, 0, 0}
		if moveDirs & input.MoveForward != 0 {
			moveVector = moveVector.Add(env.Cam.Forward)
		}else if moveDirs & input.MoveBackward != 0 {
			moveVector = moveVector.Sub(env.Cam.Forward)
		}
		if moveDirs & input.MoveLeftward != 0 {
			moveVector = moveVector.Add(env.Cam.Left)
		}else if moveDirs & input.MoveRightward != 0 {
			moveVector = moveVector.Sub(env.Cam.Left)
		}
		if moveDirs & input.MoveUpward != 0 {
			moveVector = moveVector.Add(env.Cam.Up)
		}else if moveDirs & input.MoveDownward != 0 {
			moveVector = moveVector.Sub(env.Cam.Up)
		}
		
		// If the camera needs to move, move it.
		if !moveVector.Zero() {
			env.Cam.Pos = env.Cam.Pos.Add(moveVector.Norm().Scale(0.1))
		}
		
		// If the camera needs to rotate, rotate it.
		// Note: we normalize the two vectors we change to prevent small errors from building up.
		if yaw != 0.0 {
			env.Cam.Forward = env.Cam.Forward.Rotate(env.Cam.Up, yaw * env.Cam.Fov / 2.0).Norm()
			
			// In order to prevent error buildup, we need to calculate left with respect to the global up vector.
			// First, we need to ensure that the forward vector is not parallel to the global up.
			if env.Cam.Forward.Cross(state.GlobalUp).Zero() {
				// The forward vector is parallel to the global up, so we need to nudge it slightly to prevent our camera's coordinate system from collapsing.
				xNudge := 0.0001 * float64(rand.Intn(3) - 1)
				yNudge := 0.0001 * float64(rand.Intn(3) - 1)
				zNudge := 0.0001 * float64(rand.Intn(3) - 1)
				if xNudge == 0.0 && yNudge == 0.0 && zNudge == 0.0 {
					if rand.Intn(2) == 0 {
						switch rand.Intn(3) {
						case 0:
							xNudge = 0.0001
							break
						case 1:
							yNudge = 0.0001
							break
						default:
							zNudge = 0.0001
							break
						}
					}else{
						switch rand.Intn(3) {
						case 0:
							xNudge = -0.0001
							break
						case 1:
							yNudge = -0.0001
							break
						default:
							zNudge = -0.0001
							break
						}
					}
				}
				env.Cam.Forward = env.Cam.Forward.Add(geom.Vector{xNudge, yNudge, zNudge})
			}
			
			// Now that we're sure forward and the global up are not parallel, compute left.
			env.Cam.Left = env.Cam.Forward.Cross(state.GlobalUp).Norm()
		}
		if pitch != 0.0 {
			env.Cam.Forward = env.Cam.Forward.Rotate(env.Cam.Left, pitch * (float64(surface.H) / float64(surface.W)) * env.Cam.Fov / 2.0).Norm()
			env.Cam.Up = env.Cam.Left.Cross(env.Cam.Forward).Norm()
		}
		
		// Draw the screen.
		draw(window, surface, env)
		
		// If there's still time before the next frame needs to be drawn, wait.
		currentUpdate = sdl.GetTicks()
		if currentUpdate - prevUpdate < screen.MsPerFrame {
			sdl.Delay(screen.MsPerFrame - (currentUpdate - prevUpdate))
		}
	}
}