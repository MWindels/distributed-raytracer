// Package state provides shared state information for use by workers and the master.
package state

import "github.com/mwindels/distributed-raytracer/shared/geom"

// RGB represents a colour with red, green, and blue channels.
type RGB struct {
	R, G, B uint8
}

// RGBA returns the three colour channels of an RGB object, and 0 for the alpha channel.
// This function allows RGB objects to be used with the Color (image/color) interface.
func (rgb RGB) RGBA() (uint32, uint32, uint32, uint32) {
	return uint32(rgb.R), uint32(rgb.G), uint32(rgb.B), uint32(0)
}

// Light represents a point of light in 3-dimensional space.
type Light struct {
	Pos geom.Vector
	Col RGB
}