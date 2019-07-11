// Package geom provides shared geometry functionality for use by workers and the master.
package geom

// Triangle represents a triangle in 3-dimensional space.
type Triangle struct {
	P1 Vector
	P2 Vector
	P3 Vector
}

// Intersection returns the point of intersection between a ray and the triangle t (and true) if an intersection exists.
// If no intersection exists, false is returned instead of true.
func (t Triangle) Intersection(rOrigin, rDir Vector) (Vector, bool) {
	// Compute the triangle's normal.
	tNormal := t.P2.Sub(t.P1).Cross(t.P3.Sub(t.P1))
	
	// Make sure that the ray's direction and triangle's normal are not perpendicular.
	if tNormal.Dot(rDir) != 0.0 {
		// Compute the independent variable of the ray's parameteric representation (x from rOrigin + x * rDir).
		intersectParameter := (tNormal.Dot(t.P1) - tNormal.Dot(rOrigin)) / tNormal.Dot(rDir)
		
		// Make sure that the intersection point is ahead of the ray.
		if intersectParameter >= 0.0 {
			// Compute the intersection point of the ray and the plane defined by x.
			intersect := rOrigin.Add(rDir.Scale(intersectParameter))
			
			// Return a valid intersection if the point is inside of the triangle.
			return intersect, t.P2.Sub(t.P1).Cross(intersect.Sub(t.P1)).Dot(tNormal) >= 0.0 && t.P3.Sub(t.P2).Cross(intersect.Sub(t.P2)).Dot(tNormal) >= 0.0 && t.P1.Sub(t.P3).Cross(intersect.Sub(t.P3)).Dot(tNormal) >= 0.0
		}
	}
	return Vector{}, false
}