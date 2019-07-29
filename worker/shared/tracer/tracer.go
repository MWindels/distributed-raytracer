// Package tracer provides ray-tracing functionality shared by the distributed and sequential workers.
package tracer

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/distributed-raytracer/shared/state"
	"github.com/mwindels/rtreego"
	"math"
)

// pixelToPoint translates a pixel value (i, j) to a point on a projection plane in 3D space.
// This function assumes that the projection plane is exactly one unit away from the camera.
// The parameters i and j must be in the range [0, width) and [0, height) respectively.
func pixelToPoint(i, j, width, height int, cam state.Camera) geom.Vector {
	halfWidth, halfHeight := width / 2, height / 2
	projHalfWidth := math.Tan(cam.Fov / 2.0)
	projHalfHeight := projHalfWidth * float64(height) / float64(width)
	iOffset := cam.Left().Scale(projHalfWidth * (float64(halfWidth - i) - 0.5) / float64(halfWidth))
	jOffset := cam.Up().Scale(projHalfHeight * (float64(halfHeight - j) - 0.5) / float64(halfHeight))
	return cam.Pos.Add(cam.Forward()).Add(iOffset).Add(jOffset)
}

// trace traces a single ray with a position and a direction.
// This function returns the nearest intersection point, and an associated normal vector and material.
// The last return value is whether an intersection exists.
func trace(rOrigin, rDir geom.Vector, env *state.Environment) (geom.Vector, geom.Vector, state.Material, bool) {
	nearestExists := false
	var nearestDistance float64
	var nearestIntersect, nearestNormal geom.Vector
	var nearestMaterial state.Material
	for _, s := range env.Objs.SearchCondition(func(nbb *rtreego.Rect) bool {return geom.NewBox(nbb).Intersect(rOrigin, rDir)}) {
		// Convert the rtreego.Spatial s to an object.
		o := s.(*state.Object)
		
		// Check if the ray intersects this object.
		if intersect, normal, material, hit := o.Intersection(rOrigin, rDir); hit {
			intersectDistance := intersect.Sub(env.Cam.Pos).Len()
			if !nearestExists || intersectDistance < nearestDistance {
				nearestExists = true
				nearestDistance = intersectDistance
				nearestIntersect = intersect
				nearestNormal = normal
				nearestMaterial = material
			}
		}
	}
	
	return nearestIntersect, nearestNormal, nearestMaterial, nearestExists
}

// phong calculates the colour of a point using Phong shading.
func phong(intersect, normal geom.Vector, material state.Material, env *state.Environment) colour.RGB {
	// Start by adding the ambient lighting.
	// Note: this should be multiplied by some global ambient intensity.
	colour := material.Ka
	
	// For every light, add the diffuse and specular lighting.
	// Note: the diffuse and specular intensities of a light are considered the same.
	for _, l := range env.Lights {
		lightDir := l.Pos.Sub(intersect).Norm()
		
		// Make sure the object is not in shadow.
		if shadeIntersect, _, _, shaded := trace(intersect.Add(lightDir.Scale(0.0001)), lightDir, env); !shaded || l.Pos.Sub(intersect).Len() < shadeIntersect.Sub(intersect).Len() {
			reflectDir := normal.Scale(2 * lightDir.Dot(normal)).Sub(lightDir)
			camDir := env.Cam.Pos.Sub(intersect).Norm()
			
			// Add diffuse lighting for light l.
			colour = colour.Add(material.Kd.Scale(math.Max(lightDir.Dot(normal), 0.0)).Multiply(l.Col))
			
			// Add specular lighting for light l.
			colour = colour.Add(material.Ks.Scale(math.Pow(math.Max(reflectDir.Dot(camDir), 0.0), material.Ns)).Multiply(l.Col))
		}
	}
	
	return colour
}

// Trace traces a single ray through the pixel (i, j) and into a scene.
// The parameters i and j must be in the ranges [0, width) and [0, height) respectively.
func Trace(i, j, width, height int, env *state.Environment) (colour.RGB, bool) {
	// Find the centre of the pixel (i, j) on the projection plane.
	screenIntersect := pixelToPoint(i, j, width, height, env.Cam)
	
	// If an object was hit, return a colour.
	if intersect, normal, material, valid := trace(env.Cam.Pos, screenIntersect.Sub(env.Cam.Pos).Norm(), env); valid {
		return phong(intersect, normal, material, env), true
	}else{
		return colour.RGB{}, false
	}
}