// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/rtreego"
	"github.com/mwindels/gwob"
	"math"
	"fmt"
)

// This constant is the lowest possible size of a bounding box in any dimension.
const boundEpsilon float64 = 0.0001

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
	
	obj *Object			// A pointer to the object this face resides within.
}

// Bounds gets the rectangular bounding box containing the face f.
func (f face) Bounds() *rtreego.Rect {
	// Find the smallest and largest X coordinates.
	xMin := math.Min(f.obj.vertices[f.verts[0]].X, math.Min(f.obj.vertices[f.verts[1]].X, f.obj.vertices[f.verts[2]].X))
	xMax := math.Max(f.obj.vertices[f.verts[0]].X, math.Max(f.obj.vertices[f.verts[1]].X, f.obj.vertices[f.verts[2]].X))
	
	// Find the smallest and largest Y coordinates.
	yMin := math.Min(f.obj.vertices[f.verts[0]].Y, math.Min(f.obj.vertices[f.verts[1]].Y, f.obj.vertices[f.verts[2]].Y))
	yMax := math.Max(f.obj.vertices[f.verts[0]].Y, math.Max(f.obj.vertices[f.verts[1]].Y, f.obj.vertices[f.verts[2]].Y))
	
	// Find the smallest and largest Z coordinates.
	zMin := math.Min(f.obj.vertices[f.verts[0]].Z, math.Min(f.obj.vertices[f.verts[1]].Z, f.obj.vertices[f.verts[2]].Z))
	zMax := math.Max(f.obj.vertices[f.verts[0]].Z, math.Max(f.obj.vertices[f.verts[1]].Z, f.obj.vertices[f.verts[2]].Z))
	
	// Create the bounding box.
	bbox, err := rtreego.NewRect(rtreego.Point{xMin, yMin, zMin}, []float64{math.Max(xMax - xMin, boundEpsilon), math.Max(yMax - yMin, boundEpsilon), math.Max(zMax - zMin, boundEpsilon)})
	if err != nil {
		panic(err)
	}
	
	return bbox
}

// Object represents a triangulated (3D) polygonal mesh with various material properties.
type Object struct {
	vertices []geom.Vector		// The vertices of this object.
	vertexNormals []geom.Vector	// The vertex normals of this object.
	faces *rtreego.Rtree		// Stores each of this object's triangular faces.
	
	materials []Material		// The materials of this object.
	
	Pos geom.Vector
}

// StoredObject is used to (un)marshal object data to/from the JSON format.
type StoredObject struct {
	Model string	`json:"model"`
	Pos geom.Vector	`json:"pos"`
}

// ObjectFromFile returns a new Object based on a provided OBJ file.
func ObjectFromFile(path string) (*Object, error) {
	options := gwob.ObjParserOptions{LogStats: true, Logger: func(s string) {fmt.Println(s)}, IgnoreNormals: false}
	
	// Read in the object from the file.
	inputObj, err := gwob.NewObjFromFile(path, &options)
	if err != nil {
		return nil, err
	}
	
	// Read in the material associated with the object.
	inputMatlib := gwob.NewMaterialLib()
	if len(inputObj.Mtllib) > 0 {
		inputMatlib, err = gwob.ReadMaterialLibFromFile(relativePath(path, inputObj.Mtllib), &options)
		if err != nil {
			// If the material can't be found at the relative path, try the absolute path.
			inputMatlib, err = gwob.ReadMaterialLibFromFile(inputObj.Mtllib, &options)
			if err != nil {
				return nil, err
			}
		}
	}
	
	vertexStride := inputObj.StrideSize / 4
	vertexOffset := inputObj.StrideOffsetPosition / 4
	vertexNormalOffset := inputObj.StrideOffsetNormal / 4
	
	// Initialize the object.
	obj := new(Object)
	obj.vertices = make([]geom.Vector, 0, len(inputObj.Coord) / vertexStride)
	obj.materials = make([]Material, 0, len(inputObj.Groups))
	obj.faces = rtreego.NewTree(3, 2, 5)
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
			fFace := face{}
			
			// Add the material index and object point.
			fFace.mat = matIndex
			fFace.obj = obj
			
			// Add the vertex and vertex normal indices (if they exist).
			for v := 0; v < 3; v++ {
				vIndex := g.IndexBegin + (3 * f + v)
				
				// Add the new vertex
				vVertex := geom.Vector{inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexOffset), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexOffset + 1), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexOffset + 2)}
				if vVertexIndex, exists := vertexMap[vVertex]; exists {
					fFace.verts[v] = vVertexIndex
				}else{
					fFace.verts[v] = uint(len(obj.vertices))
					vertexMap[vVertex] = uint(len(obj.vertices))
					obj.vertices = append(obj.vertices, vVertex)
				}
				
				// Add the new vertex normal (if it exists).
				if inputObj.NormCoordFound {
					vVertexNormal := geom.Vector{inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexNormalOffset), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexNormalOffset + 1), inputObj.Coord64(vertexStride * inputObj.Indices[vIndex] + vertexNormalOffset + 2)}
					if vVertexNormalIndex, exists := vertexNormalMap[vVertexNormal]; exists {
						fFace.vertNorms[v] = vVertexNormalIndex
					}else{
						fFace.vertNorms[v] = uint(len(obj.vertexNormals))
						vertexNormalMap[vVertexNormal] = uint(len(obj.vertexNormals))
						obj.vertexNormals = append(obj.vertexNormals, vVertexNormal.Norm())
					}
				}
			}
			
			// Insert the new face into the R-Tree.
			obj.faces.Insert(fFace)
		}
	}
	
	return obj, nil
}

// Bounds gets the rectangular bounding box containing the object o.
func (o Object) Bounds() *rtreego.Rect {
	// Set up a minimal bounding box.
	// Note: because we use o.Pos, we must rebuild the environment's R-Tree every time an object moves!
	xMin, xMax := o.Pos.X, o.Pos.X
	yMin, yMax := o.Pos.Y, o.Pos.Y
	zMin, zMax := o.Pos.Z, o.Pos.Z
	
	// For each vertex in the object, expand the box if necessary.
	for _, v := range o.vertices {
		xMin = math.Min(xMin, o.Pos.X + v.X)
		xMax = math.Max(xMax, o.Pos.X + v.X)
		
		yMin = math.Min(yMin, o.Pos.Y + v.Y)
		yMax = math.Max(yMax, o.Pos.Y + v.Y)
		
		zMin = math.Min(zMin, o.Pos.Z + v.Z)
		zMax = math.Max(zMax, o.Pos.Z + v.Z)
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
	
	// Compute the points of intersection.
	for _, s := range o.faces.SearchCondition(func(nbb *rtreego.Rect) bool {return geom.NewBox(nbb).Intersect(rOrigin, rDir)}) {
		// Convert the rtreego.Spatial s to a face.
		f := s.(face)
		
		// Build a triangle.
		tri := geom.Triangle{P1: o.vertices[f.verts[0]], P2: o.vertices[f.verts[1]], P3: o.vertices[f.verts[2]]}
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
	
	return nearestIntersect.Add(o.Pos), nearestVertexNormal, nearestMaterial, hasNearest
}