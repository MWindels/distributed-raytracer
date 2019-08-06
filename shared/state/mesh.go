// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/rtreego"
	"github.com/mwindels/gwob"
	"encoding/gob"
	"bytes"
	"math"
	"log"
)

func init() {
	gob.Register(face{})
	gob.Register(Mesh{})
}

// face contains a set of indices used to refer to various parts of a mesh.
type face struct {
	verts [3]uint		// The indices of each vertex of the face.
	vertNorms [3]uint	// The indices of each vertex normal of the face.
	mat uint			// The index of the material used by the face.
	
	mesh *Mesh			// A pointer to the mesh this face resides within.
}

// Bounds gets the rectangular bounding box containing the face f.
func (f face) Bounds() *rtreego.Rect {
	// Find the smallest and largest X coordinates.
	xMin := math.Min(f.mesh.vertices[f.verts[0]].X, math.Min(f.mesh.vertices[f.verts[1]].X, f.mesh.vertices[f.verts[2]].X))
	xMax := math.Max(f.mesh.vertices[f.verts[0]].X, math.Max(f.mesh.vertices[f.verts[1]].X, f.mesh.vertices[f.verts[2]].X))
	
	// Find the smallest and largest Y coordinates.
	yMin := math.Min(f.mesh.vertices[f.verts[0]].Y, math.Min(f.mesh.vertices[f.verts[1]].Y, f.mesh.vertices[f.verts[2]].Y))
	yMax := math.Max(f.mesh.vertices[f.verts[0]].Y, math.Max(f.mesh.vertices[f.verts[1]].Y, f.mesh.vertices[f.verts[2]].Y))
	
	// Find the smallest and largest Z coordinates.
	zMin := math.Min(f.mesh.vertices[f.verts[0]].Z, math.Min(f.mesh.vertices[f.verts[1]].Z, f.mesh.vertices[f.verts[2]].Z))
	zMax := math.Max(f.mesh.vertices[f.verts[0]].Z, math.Max(f.mesh.vertices[f.verts[1]].Z, f.mesh.vertices[f.verts[2]].Z))
	
	// Create the bounding box.
	bbox, err := rtreego.NewRect(rtreego.Point{xMin, yMin, zMin}, []float64{math.Max(xMax - xMin, boundEpsilon), math.Max(yMax - yMin, boundEpsilon), math.Max(zMax - zMin, boundEpsilon)})
	if err != nil {
		panic(err)
	}
	
	return bbox
}

// MarshalBinary converts a face into a binary representation.
func (f face) MarshalBinary() ([]byte, error) {
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the face's vertex, vertex normal, and material indices.
	// We don't store the mesh pointer, because it means nothing without the mesh.
	if err := encoder.Encode(f.verts); err != nil {
		return nil, err
	}
	if err := encoder.Encode(f.vertNorms); err != nil {
		return nil, err
	}
	if err := encoder.Encode(f.mat); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives a face from its binary representation.
func (f *face) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Decode the face's vertex, vertex normal, and material indices.
	if err := decoder.Decode(&f.verts); err != nil {
		return err
	}
	if err := decoder.Decode(&f.vertNorms); err != nil {
		return err
	}
	if err := decoder.Decode(&f.mat); err != nil {
		return err
	}
	
	return nil
}

// Material represents the material properties of one or more faces.
type Material struct {
	Ka, Kd, Ks colour.RGB	// The ambient, diffuse, and specular intensities respectively.
	Ns float64				// The specular exponent.
}

// Mesh represents a triangulated (3D) polygonal mesh with various material properties.
type Mesh struct {
	vertices []geom.Vector		// The vertices of this mesh.
	vertexNormals []geom.Vector	// The vertex normals of this mesh.
	faces *rtreego.Rtree		// Stores each of this mesh's triangular faces.
	
	materials []Material		// The materials of this mesh.
}

// MeshFromFile returns a new mesh based on a provided Wavefront OBJ file.
func MeshFromFile(path string) (*Mesh, error) {
	options := gwob.ObjParserOptions{LogStats: true, Logger: func(s string) {log.Println(s)}, IgnoreNormals: false}
	
	// Read in the mesh from the file.
	inputMesh, err := gwob.NewObjFromFile(path, &options)
	if err != nil {
		return nil, err
	}
	
	// Read in the material library associated with the mesh.
	inputMatlib := gwob.NewMaterialLib()
	if len(inputMesh.Mtllib) > 0 {
		inputMatlib, err = gwob.ReadMaterialLibFromFile(relativePath(path, inputMesh.Mtllib), &options)
		if err != nil {
			// If the material can't be found at the relative path, try the absolute path.
			inputMatlib, err = gwob.ReadMaterialLibFromFile(inputMesh.Mtllib, &options)
			if err != nil {
				return nil, err
			}
		}
	}
	
	vertexStride := inputMesh.StrideSize / 4
	vertexOffset := inputMesh.StrideOffsetPosition / 4
	vertexNormalOffset := inputMesh.StrideOffsetNormal / 4
	
	// Initialize the mesh.
	mesh := &Mesh{
		vertices: make([]geom.Vector, 0, len(inputMesh.Coord) / vertexStride),
		materials: make([]Material, 0, len(inputMesh.Groups)),
		faces: rtreego.NewTree(3, 2, 5),
	}
	if inputMesh.NormCoordFound {
		mesh.vertexNormals = make([]geom.Vector, 0, len(inputMesh.Coord) / vertexStride)
	}
	
	// Assemble the mesh.
	vertexMap := make(map[geom.Vector]uint)
	vertexNormalMap := make(map[geom.Vector]uint)
	materialMap := make(map[Material]uint)
	for _, g := range inputMesh.Groups {
		// Assign a default material.
		mat := Material{Ka: colour.NewRGB(0x10, 0x10, 0x10), Kd: colour.NewRGB(0xFF, 0xFF, 0xFF), Ks: colour.NewRGB(0x00, 0x00, 0x00), Ns: 0.0}
		if gMat, exists := inputMatlib.Lib[g.Usemtl]; exists {
			// If a material exists for this group, use it instead.
			mat = Material{Ka: colour.NewRGBFromFloats(gMat.Ka[0], gMat.Ka[1], gMat.Ka[2]), Kd: colour.NewRGBFromFloats(gMat.Kd[0], gMat.Kd[1], gMat.Kd[2]), Ks: colour.NewRGBFromFloats(gMat.Ks[0], gMat.Ks[1], gMat.Ks[2]), Ns: float64(gMat.Ns)}
		}
		
		// If the material is new, add it.
		matIndex, exists := materialMap[mat]
		if !exists {
			matIndex = uint(len(mesh.materials))
			mesh.materials = append(mesh.materials, mat)
			materialMap[mat] = matIndex
		}
		
		// Fill the vertex and vertex normal slices.
		for f := 0; f < g.IndexCount / 3; f++ {
			fFace := face{
				mat: matIndex,
				mesh: mesh,
			}
			
			// Add the vertex and vertex normal indices (if they exist).
			for v := 0; v < 3; v++ {
				vIndex := g.IndexBegin + (3 * f + v)
				vVertex := geom.Vector{
					inputMesh.Coord64(vertexStride * inputMesh.Indices[vIndex] + vertexOffset),
					inputMesh.Coord64(vertexStride * inputMesh.Indices[vIndex] + vertexOffset + 1),
					inputMesh.Coord64(vertexStride * inputMesh.Indices[vIndex] + vertexOffset + 2),
				}
				
				// Add the new vertex.
				if vVertexIndex, exists := vertexMap[vVertex]; exists {
					fFace.verts[v] = vVertexIndex
				}else{
					fFace.verts[v] = uint(len(mesh.vertices))
					vertexMap[vVertex] = uint(len(mesh.vertices))
					mesh.vertices = append(mesh.vertices, vVertex)
				}
				
				// Add the new vertex normal (if it exists).
				if inputMesh.NormCoordFound {
					vVertexNormal := geom.Vector{
						inputMesh.Coord64(vertexStride * inputMesh.Indices[vIndex] + vertexNormalOffset),
						inputMesh.Coord64(vertexStride * inputMesh.Indices[vIndex] + vertexNormalOffset + 1),
						inputMesh.Coord64(vertexStride * inputMesh.Indices[vIndex] + vertexNormalOffset + 2),
					}
					if vVertexNormalIndex, exists := vertexNormalMap[vVertexNormal]; exists {
						fFace.vertNorms[v] = vVertexNormalIndex
					}else{
						fFace.vertNorms[v] = uint(len(mesh.vertexNormals))
						vertexNormalMap[vVertexNormal] = uint(len(mesh.vertexNormals))
						mesh.vertexNormals = append(mesh.vertexNormals, vVertexNormal.Norm())
					}
				}
			}
			
			// Insert the new face into the R-Tree.
			mesh.faces.Insert(fFace)
		}
	}
	
	return mesh, nil
}

// MarshalBinary converts a mesh into a binary representation.
func (m Mesh) MarshalBinary() ([]byte, error) {
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the mesh's vertices, vertex normals, faces, and materials.
	if err := encoder.Encode(m.vertices); err != nil {
		return nil, err
	}
	if err := encoder.Encode(m.vertexNormals); err != nil {
		return nil, err
	}
	if err := encoder.Encode(m.faces.SearchCondition(func(nbb *rtreego.Rect) bool {return true})); err != nil {
		return nil, err
	}
	if err := encoder.Encode(m.materials); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives a mesh from its binary representation.
func (m *Mesh) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Decode the mesh's vertices, vertex normals, faces, and materials.
	var faces []rtreego.Spatial
	if err := decoder.Decode(&m.vertices); err != nil {
		return err
	}
	if err := decoder.Decode(&m.vertexNormals); err != nil {
		return err
	}
	if err := decoder.Decode(&faces); err != nil {
		return err
	}
	if err := decoder.Decode(&m.materials); err != nil {
		return err
	}
	
	// Rebuild an R-Tree for the faces.
	m.faces = rtreego.NewTree(3, 2, 5)
	
	// Because our faces have a mesh associated with them, we need to add a pointer to that mesh.
	// Then, add the face value to the faces R-Tree.
	for _, s := range faces {
		f := s.(face)
		f.mesh = m
		
		m.faces.Insert(f)
	}
	
	return nil
}