// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/rtreego"
	"encoding/gob"
	"bytes"
	"math"
)

func init() {
	gob.Register(Object{})
}

// Object represents an instance of a mesh in 3D space.
type Object struct {
	Pos geom.Vector	// The position of the object.
	
	id uint			// An unsigned integer that uniquely identifies this object (used by an environment to retrieve a mesh pointer).
	mesh *Mesh		// The unit mesh which represents this object (means nothing without an environment).
}

// StoredObject is used to (un)marshal object data to/from the JSON format.
type StoredObject struct {
	Model string	`json:"model"`
	Pos geom.Vector	`json:"pos"`
}

// Bounds gets the rectangular bounding box containing the object o.
func (o Object) Bounds() *rtreego.Rect {
	// Set up a minimal bounding box.
	// Note: because we use o.Pos, we must rebuild the environment's R-Tree every time an object moves!
	xMin, xMax := o.Pos.X, o.Pos.X
	yMin, yMax := o.Pos.Y, o.Pos.Y
	zMin, zMax := o.Pos.Z, o.Pos.Z
	
	// For each vertex in the object's mesh, expand the box if necessary.
	if o.mesh != nil {
		for _, v := range o.mesh.vertices {
			xMin = math.Min(xMin, o.Pos.X + v.X)
			xMax = math.Max(xMax, o.Pos.X + v.X)
			
			yMin = math.Min(yMin, o.Pos.Y + v.Y)
			yMax = math.Max(yMax, o.Pos.Y + v.Y)
			
			zMin = math.Min(zMin, o.Pos.Z + v.Z)
			zMax = math.Max(zMax, o.Pos.Z + v.Z)
		}
	}
	
	// Create the bounding box.
	bbox, err := rtreego.NewRect(rtreego.Point{xMin, yMin, zMin}, []float64{math.Max(xMax - xMin, boundEpsilon), math.Max(yMax - yMin, boundEpsilon), math.Max(zMax - zMin, boundEpsilon)})
	if err != nil {
		panic(err)
	}
	
	return bbox
}

// Intersection computes the intersection between a ray and an object.
// This function's return values are: (1) the point of intersection, (2) the normal vector at that point, (3) the material at that point, and (4) whether or not the ray intersected the object.
func (o Object) Intersection(rOrigin, rDir geom.Vector) (geom.Vector, geom.Vector, Material, bool) {
	hasNearest := false
	var nearestDistance float64
	var nearestIntersect geom.Vector
	var nearestVertexNormal geom.Vector
	var nearestMaterial Material
	
	// Offset the ray to compensate for the object's position.
	rOrigin = rOrigin.Sub(o.Pos)
	
	m := o.mesh
	if m != nil {
		// Compute the points of intersection with respect to the object's unit mesh.
		for _, s := range m.faces.SearchCondition(func(nbb *rtreego.Rect) bool {return geom.NewBox(nbb).Intersect(rOrigin, rDir)}) {
			// Convert the rtreego.Spatial s to a face.
			f := s.(face)
			
			// Build a triangle.
			tri := geom.Triangle{P1: m.vertices[f.verts[0]], P2: m.vertices[f.verts[1]], P3: m.vertices[f.verts[2]]}
			if len(m.vertexNormals) > 0 {
				tri.N1 = m.vertexNormals[f.vertNorms[0]]
				tri.N2 = m.vertexNormals[f.vertNorms[1]]
				tri.N3 = m.vertexNormals[f.vertNorms[2]]
			}
			
			// Find the intersection of the ray and the triangle.
			if intersect, bcoords, hit := tri.Intersection(rOrigin, rDir); hit {
				var normal geom.Vector
				if len(m.vertexNormals) > 0 {
					normal = tri.InterpNormal(bcoords)
				}else{
					normal = tri.Normal()
				}
				
				intersectDistance := rOrigin.Sub(intersect).Len()
				if !hasNearest || intersectDistance < nearestDistance {
					hasNearest = true
					nearestDistance = intersectDistance
					nearestIntersect = intersect
					nearestVertexNormal = normal
					nearestMaterial = m.materials[f.mat]
				}
			}
		}
	}
	
	return nearestIntersect.Add(o.Pos), nearestVertexNormal, nearestMaterial, hasNearest
}

// MarshalBinary converts an object into a binary representation.
func (o Object) MarshalBinary() ([]byte, error) {
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the object's position, and id.
	if err := encoder.Encode(o.Pos); err != nil {
		return nil, err
	}
	if err := encoder.Encode(o.id); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives an object from its binary representation.
func (o *Object) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Decode the object's position, and id.
	if err := decoder.Decode(&o.Pos); err != nil {
		return err
	}
	if err := decoder.Decode(&o.id); err != nil {
		return err
	}
	
	// For now, set the mesh pointer to nil.
	// To get a mesh pointer, LinkTo() will need to be called with an EnvMutables containing this object.
	o.mesh = nil
	
	return nil
}