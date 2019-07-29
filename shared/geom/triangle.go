// Package geom provides shared geometry objects for use by workers and the master.
package geom

// Triangle represents a triangle in 3-dimensional space.
// Note that the points are assumed to be stored in counter-clockwise order.
type Triangle struct {
	P1 Vector
	P2 Vector
	P3 Vector
	
	N1 Vector	// The vertex normal for P1.
	N2 Vector	// The vertex normal for P2.
	N3 Vector	// The vertex normal for P3.
}

// BaryCoords represents the barycentric coordinates of some point (I) on a triangle.
type BaryCoords struct {
	R1 float64	// The ratio between the signed area of triangle (I, P2, P3) and the area of the triangle (P1, P2, P3).
	R2 float64	// The ratio between the signed area of triangle (I, P3, P1) and the area of the triangle (P1, P2, P3).
	R3 float64	// The ratio between the signed area of triangle (I, P1, P2) and the area of the triangle (P1, P2, P3).
}

// Normal computes the normal vector of the triangle t.
func (t Triangle) Normal() Vector {
	return t.P2.Sub(t.P1).Cross(t.P3.Sub(t.P1)).Norm()
}

// InterpNormal computes the normal vector at a point (defined by barycentric coordinates) on the triangle t by interpolating with respect to t's vertex normals.
func (t Triangle) InterpNormal(p BaryCoords) Vector {
	return t.N1.Scale(p.R1).Add(t.N2.Scale(p.R2)).Add(t.N3.Scale(p.R3)).Norm()
}

// Intersection returns the point of intersection between a ray and a triangle t.
// Barycentric coordinates are also returned if an intersection point exists.
// If no intersection exists, then the last value returned will be false.
// Note that this is essentially the Möller-Trumbore algorithm.
func (t Triangle) Intersection(rOrigin, rDir Vector) (Vector, BaryCoords, bool) {
	p1p2, p1p3, negativeDir := t.P2.Sub(t.P1), t.P3.Sub(t.P1), rDir.Scale(-1)
	
	// Compute the cosine of the angle between t's normal and the direction of the ray using the scalar triple product.
	// This is equivalent to the determinant of the matrix composed of the three vectors.
	incidence := p1p2.Dot(p1p3.Cross(negativeDir))
	
	// If the cosine of the angle of incidence is non-zero, then the ray will intersect the plane of the triangle.
	// Then, we'll use Cramer's rule (and scalar triple products instead of determinants) to compute the barycentric coordinates.
	if incidence != 0.0 {
		p1Or := rOrigin.Sub(t.P1)
		
		// Compute the ratio for the triangle defined by all points except P2.
		r2 := p1Or.Dot(p1p3.Cross(negativeDir)) / incidence
		
		// If the ratio is positive and within [0, 1], continue.
		if 0.0 <= r2 && r2 <= 1.0 {
			// Compute the ratio for the triangle defined by all points except P3.
			r3 := p1p2.Dot(p1Or.Cross(negativeDir)) / incidence
			
			// If the sum of the ratios is positive and within [0, 1], continue.
			if 0.0 <= r2 + r3 && r2 + r3 <= 1.0 {
				// Compute the ratio for the triangle defined by all points except P1.
				r1 := 1.0 - r2 - r3
				
				// If all barycentric coordinates are non-negative, then t has been intersected.
				if r1 >= 0.0 && r2 >= 0.0 && r3 >= 0.0 {
					// Compute the amount by which the ray's direction has to be scaled to hit the triangle's plane.
					dirScale := p1p2.Dot(p1p3.Cross(p1Or)) / incidence
					
					// Ensure that the intersection point is in front of the ray.
					if dirScale >= 0.0 {
						return rOrigin.Add(rDir.Scale(dirScale)), BaryCoords{R1: r1, R2: r2, R3: r3}, true
					}
				}
			}
		}
	}
	
	return Vector{}, BaryCoords{}, false
}