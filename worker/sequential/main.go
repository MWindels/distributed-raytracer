package main

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/distributed-raytracer/shared/screen"
	"github.com/mwindels/distributed-raytracer/shared/input"
	"github.com/mwindels/distributed-raytracer/worker/shared/tracer"
	"github.com/mwindels/rtreego"
	"image/color"
	"math/rand"
	"time"
	"math"
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
	
	// Start the screen.
	window, surface := screen.StartScreen("Sequential Ray-Tracer", 960/4, 540/4)
	defer screen.StopScreen(window)
	
	// Load some objects in.
	obj1, err := state.ObjectFromFile("capsule.obj")
	obj1.Pos = geom.Vector{1.0, 1.0, -1.0}
	if err != nil {
		panic(err)
	}
	obj2, err := state.ObjectFromFile("monkey.obj")
	obj2.Pos = geom.Vector{1.0, 1.0, 2.0}
	if err != nil {
		panic(err)
	}
	
	// Create an environment (should be able to load this from a JSON file or something).
	env := state.Environment{
		Objs: rtreego.NewTree(3, 2, 5, obj1, obj2),
		Lights: []state.Light{
			state.Light{Pos: geom.Vector{0, 3.0, 10.0}, Col: colour.NewRGB(0xB0, 0xB0, 0xB0)},
			//state.Light{Pos: geom.Vector{0, -3.0, 10.0}, Col: colour.NewRGB(0x80, 0x40, 0x40)},
			//state.Light{Pos: geom.Vector{0, 0.0, -10.0}, Col: colour.NewRGB(0x60, 0x60, 0x60)},
		},
		Cam: state.NewCamera(geom.Vector{1, 1, 5}, geom.Vector{0, 0, -1}, math.Pi / 3.0),
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
			
			// Furthermore, recompute up with respect to left (with indirect respect to the global up vector) so error doesn't build up in forward on the next yaw.
			env.Cam.Up = env.Cam.Left.Cross(env.Cam.Forward).Norm()
		}
		if pitch != 0.0 {
			env.Cam.Forward = env.Cam.Forward.Rotate(env.Cam.Left, pitch * (float64(surface.H) / float64(surface.W)) * env.Cam.Fov / 2.0).Norm()
			env.Cam.Up = env.Cam.Left.Cross(env.Cam.Forward).Norm()
		}
		
		// Draw the screen.
		draw(window, surface, &env)
		
		// If there's still time before the next frame needs to be drawn, wait.
		currentUpdate = sdl.GetTicks()
		if currentUpdate - prevUpdate < screen.MsPerFrame {
			sdl.Delay(screen.MsPerFrame - (currentUpdate - prevUpdate))
		}
	}
}