// Package geom provides shared geometry objects for use by workers and the master.
package geom

import "github.com/mwindels/rtreego"

// This array contains the normal vectors for the six sides of an axis-aligned 3D box.
// This should be const, but Go doesn't let us have const structs.  Treat it as read-only.
var boxNormals [6]Vector = [6]Vector{
	Vector{1, 0, 0}, Vector{-1, 0, 0},
	Vector{0, 1, 0}, Vector{0, -1, 0},
	Vector{0, 0, 1}, Vector{0, 0, -1},
}

// Box represents a rectangular 3-dimensional axis-aligned box.
type Box struct {
	MinCorner Vector	// The position of the corner with the smallest coordinate values.
	MaxCorner Vector	// The position of the corner with the largest coordinate values.
}

// NewBox creates a new box from an R-Tree's bounding box.
func NewBox(bbox *rtreego.Rect) Box {
	return Box{
		MinCorner: Vector{bbox.PointCoord(0), bbox.PointCoord(1), bbox.PointCoord(2)},
		MaxCorner: Vector{bbox.PointCoord(0) + bbox.LengthsCoord(0), bbox.PointCoord(1) + bbox.LengthsCoord(1), bbox.PointCoord(2) + bbox.LengthsCoord(2)},
	}
}

// Intersect determines whether a ray intersects the box b.
func (b Box) Intersect(rOrigin, rDir Vector) bool {
	// For each side of the box...
	for _, sNormal := range boxNormals {
		// Check to make sure the ray is not perpendicular to the side's normal.
		if rDir.Dot(sNormal) != 0.0 {
			// Find a point on the side's plane.
			var sPoint Vector
			if sNormal.Dot(Vector{1, 1, 1}) < 0 {
				sPoint = b.MinCorner
			}else{
				sPoint = b.MaxCorner
			}
			
			// Compute the amount by which the ray's direction has to be scaled to hit the side's plane.
			dirScale := sPoint.Sub(rOrigin).Dot(sNormal) / rDir.Dot(sNormal)
			
			// Ensure that the intersection point is in front of the ray.
			if dirScale >= 0.0 {
				// Compute the point of intersection.
				intersect := rOrigin.Add(rDir.Scale(dirScale))
				
				// If the intersection point is within the rectangle on the side's plane, return true.
				if sNormal.X != 0.0 {
					if (b.MinCorner.Y <= intersect.Y && intersect.Y <= b.MaxCorner.Y) && (b.MinCorner.Z <= intersect.Z && intersect.Z <= b.MaxCorner.Z) {
						return true
					}
				}else if sNormal.Y != 0.0 {
					if (b.MinCorner.X <= intersect.X && intersect.X <= b.MaxCorner.X) && (b.MinCorner.Z <= intersect.Z && intersect.Z <= b.MaxCorner.Z) {
						return true
					}
				}else if sNormal.Z != 0.0 {
					if (b.MinCorner.X <= intersect.X && intersect.X <= b.MaxCorner.X) && (b.MinCorner.Y <= intersect.Y && intersect.Y <= b.MaxCorner.Y) {
						return true
					}
				}
			}
		}
	}
	
	return false
}