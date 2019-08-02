// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"encoding/gob"
	"math/rand"
	"bytes"
	"math"
	"time"
	"fmt"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	gob.Register(Camera{})
}

// Camera represents a camera in 3-dimensional space.
type Camera struct {
	Pos geom.Vector
	forward, left, up geom.Vector	// Keep these normalized to prevent small errors from building up.
	Fov float64
}

// StoredCamera is used to (un)marshal camera data to/from the JSON format.
type StoredCamera struct {
	Pos geom.Vector	`json:"pos"`
	Dir geom.Vector	`json:"dir"`
	Fov float64		`json:"fov"`
}

// NewCamera initializes a new camera with appropriate orientation values.
// If dir is parallel to the global up vector, this function returns an error.
func NewCamera(pos, dir geom.Vector, fov float64) (Camera, error) {
	if dir.Cross(GlobalUp).Zero() {
		return Camera{}, fmt.Errorf("Camera dir is parallel to global up %v.", GlobalUp)
	}else{
		forward := dir.Norm()
		left := dir.Cross(GlobalUp).Norm()
		up := left.Cross(forward)	// This is already normalized.
		return Camera{Pos: pos, forward: forward, left: left, up: up, Fov: fov}, nil
	}
}

// Forward returns the forward vector of a camera.
func (c Camera) Forward() geom.Vector {
	return c.forward
}

// Left returns the left vector of a camera.
func (c Camera) Left() geom.Vector {
	return c.left
}

// Up returns the up vector of a camera.
func (c Camera) Up() geom.Vector {
	return c.up
}

// Move moves a camera some distance in some combination of directions.
func (c *Camera) Move(distance float64, forward, backward, leftward, rightward, upward, downward bool) {
	moveDir := geom.Vector{0, 0, 0}
	
	// Set up the direction vector.
	if forward != backward {
		if forward {
			moveDir = moveDir.Add(c.forward)
		}else{
			moveDir = moveDir.Sub(c.forward)
		}
	}
	if leftward != rightward {
		if leftward {
			moveDir = moveDir.Add(c.left)
		}else{
			moveDir = moveDir.Sub(c.left)
		}
	}
	if upward != downward {
		if upward {
			moveDir = moveDir.Add(c.up)
		}else{
			moveDir = moveDir.Sub(c.up)
		}
	}
	
	// Move the camera a given distance in the given direction.
	if !moveDir.Zero() {
		c.Pos = c.Pos.Add(moveDir.Norm().Scale(distance))
	}
}

// nudgeForward offsets a camera's forward vector in a random direction by at least some
// specified value, and by at most root 3 times the specified value.
func (c *Camera) nudgeForward(nudge float64) {
	if nudge != 0.0 {
		xNudge := nudge * float64(rand.Intn(3) - 1)
		yNudge := nudge * float64(rand.Intn(3) - 1)
		zNudge := nudge * float64(rand.Intn(3) - 1)
		
		// If all the nudge values are zero, force one to be non-zero.
		if xNudge == 0.0 && yNudge == 0.0 && zNudge == 0.0 {
			var sign float64
			if rand.Intn(2) == 0 {
				sign = 1.0
			}else{
				sign = -1.0
			}
			
			switch rand.Intn(3) {
			case 0:
				xNudge = sign * nudge
				break
			case 1:
				yNudge = sign * nudge
				break
			default:
				zNudge = sign * nudge
				break
			}
		}
		
		// Nudge the forward vector.
		c.forward = c.forward.Add(geom.Vector{xNudge, yNudge, zNudge})
	}
}

// Yaw rotates a camera by theta radians about its up vector.
func (c *Camera) Yaw(theta float64) {
	if math.Mod(theta, 2.0 * math.Pi) != 0.0 {
		c.forward = c.forward.Rotate(c.up, theta).Norm()
		
		// Ensure that the forward vector is not parallel to the global up.
		if c.forward.Cross(GlobalUp).Zero() {
			c.nudgeForward(0.0001)
		}
		
		// Now that we're sure forward and the global up are not parallel, compute left.
		c.left = c.forward.Cross(GlobalUp).Norm()
		
		// We'll also recompute up with respect to left (hence with indirect respect to the global up vector).
		// This keeps error from building up in forward on the next yaw.
		c.up = c.left.Cross(c.forward).Norm()
	}
}

// Pitch rotates a camera by theta radians about its left vector.
func (c *Camera) Pitch(theta float64) {
	if math.Mod(theta, 2.0 * math.Pi) != 0.0 {
		c.forward = c.forward.Rotate(c.left, theta).Norm()
		c.up = c.left.Cross(c.forward).Norm()
	}
}

// MarshalBinary converts a camera into a binary representation.
func (c Camera) MarshalBinary() ([]byte, error) {
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the camera's position, forward vector, and fov.
	if err := encoder.Encode(c.Pos); err != nil {
		return nil, err
	}
	if err := encoder.Encode(c.forward); err != nil {
		return nil, err
	}
	if err := encoder.Encode(c.Fov); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives a camera from its binary representation.
func (c *Camera) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Decode the camera's position, forward vector, and fov.
	var pos, forward geom.Vector
	var fov float64
	if err := decoder.Decode(&pos); err != nil {
		return err
	}
	if err := decoder.Decode(&forward); err != nil {
		return err
	}
	if err := decoder.Decode(&fov); err != nil {
		return err
	}
	
	// Reconstruct the camera.
	if rebuilt, err := NewCamera(pos, forward, fov); err == nil {
		*c = rebuilt
	}else{
		return err
	}
	
	return nil
}