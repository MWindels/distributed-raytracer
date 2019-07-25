// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/rtreego"
)

// This variable represents the global up vector.
// Because Go doesn't support constant structures, this has to be a variable.
var GlobalUp geom.Vector = geom.Vector{0, 1, 0}

// Environment represents a 3-dimensional space full of objects.
type Environment struct {
	Objs *rtreego.Rtree
	Lights []Light
	Cam Camera
}