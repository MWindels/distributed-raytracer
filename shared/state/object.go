// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/gwob"
	"fmt"
)

// Material represents the material properties of one or more faces.
type Material struct {
	Ka, Kd, Ks colour.RGB	// The ambient, diffuse, and specular intensities respectively.
	Ns float64				// The specular exponent.
}

// face contains a set of indices used to refer to various parts of an Object.
type face struct {
	verts [3]uint		// The indices of each vertex of the face.
	vertNorms [3]uint	// The indices of each vertex normal of the face.
	mat uint			// The index of the material used by the face.
}

// Object represents a triangulated (3D) polygonal mesh with various material properties.
type Object struct {
	vertices []geom.Vector		// The vertices of this object.
	vertexNormals []geom.Vector	// The vertex normals of this object.
	faces []face				// Stores indices associated with each face (triangle).
	
	materials []Material
	
	Pos geom.Vector
}

// ObjectFromFile returns a new Object based on a provided OBJ file.
func ObjectFromFile(path string) (Object, error) {
	options := gwob.ObjParserOptions{LogStats: true, Logger: func(s string) {fmt.Println(s)}, IgnoreNormals: false}
	
	// Read in the object from the file.
	inputObj, err := gwob.NewObjFromFile(path, &options)
	if err != nil {
		return Object{}, err
	}
	
	// Read in the material associated with the object.
	inputMatlib := gwob.NewMaterialLib()
	if len(inputObj.Mtllib) > 0 {
		inputMatlib, err = gwob.ReadMaterialLibFromFile(inputObj.Mtllib, &options)
		if err != nil {
			return Object{}, err
		}
	}
	
	vertexStride := inputObj.StrideSize / 4
	vertexOffset := inputObj.StrideOffsetPosition / 4
	vertexNormalOffset := inputObj.StrideOffsetNormal / 4
	
	// Initialize the object.
	obj := Object{
		vertices: make([]geom.Vector, 0, len(inputObj.Coord) / vertexStride),
		faces: make([]face, len(inputObj.Indices) / 3, len(inputObj.Indices) / 3),
		materials: make([]Material, 0, len(inputObj.Groups)),
		Pos: geom.Vector{0, 0, 0},
	}
	if inputObj.NormCoordFound {
		obj.vertexNormals = make([]geom.Vector, 0, len(inputObj.Coord) / vertexStride)
	}
	
	// Assemble the object.
	vertexMap := make(map[geom.Vector]uint)
	vertexNormalMap := make(map[geom.Vector]uint)
	materialMap := make(map[Material]uint)
	for _, g := range inputObj.Groups {
		// Assign a default material.
		mat := Material{Ka: colour.NewRGB(0x10, 0x10, 0x10), Kd: colour.NewRGB(0xFF, 0xFF, 0xFF), Ks: colour.NewRGB(0x00, 0x00, 0x00), Ns: 0.0}
		if gMat, exists := inputMatlib.Lib[g.Usemtl]; exists {
			// If a material exists for this group, use it instead.
			mat = Material{Ka: colour.NewRGBFromFloats(gMat.Ka[0], gMat.Ka[1], gMat.Ka[2]), Kd: colour.NewRGBFromFloats(gMat.Kd[0], gMat.Kd[1], gMat.Kd[2]), Ks: colour.NewRGBFromFloats(gMat.Ks[0], gMat.Ks[1], gMat.Ks[2]), Ns: float64(gMat.Ns)}
		}
		
		// Add the new material.
		matIndex, exists := materialMap[mat]
		if !exists {
			matIndex = uint(len(obj.materials))
			obj.materials = append(obj.materials, mat)
			materialMap[mat] = matIndex
		}
		
		// Fill the vertex and vertex normal slices.
		for f := 0; f < g.IndexCount / 3; f++ {
			// Add the material index.
			obj.faces[g.IndexBegin / 3 + f].mat = matIndex
			
			// Add the vertex and vertex normal indices (if they exist).
			for v := 0; v < 3; v++ {
				vIndex := g.IndexBegin + (3 * f + v)
				
				// Add the new vertex
				vVertex := geom.Vector{inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexOffset), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexOffset + 1), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexOffset + 2)}
				if vVertexIndex, exists := vertexMap[vVertex]; exists {
					obj.faces[g.IndexBegin / 3 + f].verts[v] = vVertexIndex
				}else{
					vertexMap[vVertex] = uint(len(obj.vertices))
					obj.faces[g.IndexBegin / 3 + f].verts[v] = uint(len(obj.vertices))
					obj.vertices = append(obj.vertices, vVertex)
				}
				
				// Add the new vertex normal (if it exists).
				if inputObj.NormCoordFound {
					vVertexNormal := geom.Vector{inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexNormalOffset), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexNormalOffset + 1), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexNormalOffset + 2)}
					if vVertexNormalIndex, exists := vertexNormalMap[vVertexNormal]; exists {
						obj.faces[g.IndexBegin / 3 + f].vertNorms[v] = vVertexNormalIndex
					}else{
						vertexNormalMap[vVertexNormal] = uint(len(obj.vertexNormals))
						obj.faces[g.IndexBegin / 3 + f].vertNorms[v] = uint(len(obj.vertexNormals))
						obj.vertexNormals = append(obj.vertexNormals, vVertexNormal.Norm())
					}
				}
			}
		}
	}
	
	return obj, nil
}

// Intersection computes the intersection between a ray and an object.
// This function's return values are: (1) the point of intersection, (2) the normal vector at that point, (3) the material at that point, and (4) whether or not the ray intersected the object.
func (o Object) Intersection(rOrigin, rDir geom.Vector) (geom.Vector, geom.Vector, Material, bool) {
	hasNearest := false
	var nearestDistance float64
	var nearestIntersect geom.Vector
	var nearestVertexNormal geom.Vector
	var nearestMaterial Material
	
	// Compute the points of intersection.
	for _, f := range o.faces {
		// Build a triangle.
		tri := geom.Triangle{P1: o.Pos.Add(o.vertices[f.verts[0]]), P2: o.Pos.Add(o.vertices[f.verts[1]]), P3: o.Pos.Add(o.vertices[f.verts[2]])}
		if len(o.vertexNormals) > 0 {
			tri.N1 = o.vertexNormals[f.vertNorms[0]]
			tri.N2 = o.vertexNormals[f.vertNorms[1]]
			tri.N3 = o.vertexNormals[f.vertNorms[2]]
		}
		
		// Find the intersection of the ray and the triangle.
		if intersect, bcoords, hit := tri.Intersection(rOrigin, rDir); hit {
			var normal geom.Vector
			if len(o.vertexNormals) > 0 {
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
				nearestMaterial = o.materials[f.mat]
			}
		}
	}
	
	return nearestIntersect, nearestVertexNormal, nearestMaterial, hasNearest
}