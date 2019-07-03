// Package geom provides shared geometry functionality for use by workers and the master.
package geom

import "math"

// Vector represents a vector in 3-dimensional space.
type Vector struct {
	X float64
	Y float64
	Z float64
}

// Add returns the sum of vectors a and b.
func (a Vector) Add(b Vector) Vector {
	return Vector{X: a.X + b.X, Y: a.Y + b.Y, Z: a.Z + b.Z}
}

// Sub returns the difference of vectors a and b.
func (a Vector) Sub(b Vector) Vector {
	return Vector{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}

// Scale returns the vector a multiplied by the scalar s.
func (a Vector) Scale(s float64) Vector {
	return Vector{X: s * a.X, Y: s * a.Y, Z: s * a.Z}
}

// Dot returns the dot product of the vectors a and b.
func (a Vector) Dot(b Vector) float64 {
	return a.X * b.X + a.Y * b.Y + a.Z * b.Z
}

// Cross returns the cross product of the vectors a and b.
func (a Vector) Cross(b Vector) Vector {
	return Vector{X: a.Y * b.Z - a.Z * b.Y, Y: a.Z * b.X - a.X * b.Z, Z: a.X * b.Y - a.Y * b.X}
}

// Rotate returns the vector a rotated theta radians around the (normalized) vector b.
func (a Vector) Rotate(b Vector, theta float64) Vector {
	// This uses Rodrigues' rotation formula.
	return a.Scale(math.Cos(theta)).Add(b.Cross(a).Scale(math.Sin(theta))).Add(b.Scale(b.Dot(a) * (1.0 - math.Cos(theta))))
}

// Zero returns whether the vector a is a zero vector.
func (a Vector) Zero() bool {
	return a.X == 0.0 && a.Y == 0.0 && a.Z == 0.0
}

// Norm returns the normalized form of the vector a.
func (a Vector) Norm() Vector {
	mag := math.Sqrt(a.X * a.X + a.Y * a.Y + a.Z * a.Z)
	return Vector{X: a.X / mag, Y: a.Y / mag, Z: a.Z / mag}
}

// Len returns the length of the vector a.
func (a Vector) Len() float64 {
	return math.Sqrt(a.X * a.X + a.Y * a.Y + a.Z * a.Z)
}