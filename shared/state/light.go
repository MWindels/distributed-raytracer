// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
)

// Light represents a point of light in 3-dimensional space.
type Light struct {
	Pos geom.Vector
	Col colour.RGB
}

// StoredLight is used to (un)marshal light data to/from the JSON format.
type StoredLight struct {
	Pos geom.Vector			`json:"pos"`
	Col colour.StoredRGB	`json:"col"`
}