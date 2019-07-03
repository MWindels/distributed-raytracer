// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"fmt"
)

// Camera represents a camera in 3-dimensional space.
type Camera struct {
	Pos geom.Vector
	Forward, Up, Left geom.Vector	// Keep these normalized.
	Fov float64
}

// NewCamera initializes a new camera with appropriate orientation values.
// If dir is parallel to the global up vector, this function panics.
func NewCamera(pos, dir geom.Vector, fov float64) Camera {
	if dir.Cross(GlobalUp).Zero() {
		panic(fmt.Sprintf("Camera parameter dir is parallel to global up %v.", GlobalUp))
	}else{
		forward := dir.Norm()
		left := dir.Cross(GlobalUp).Norm()
		up := left.Cross(forward)	// This is already normalized.
		return Camera{Pos: pos, Forward: forward, Up: up, Left: left, Fov: fov}
	}
}