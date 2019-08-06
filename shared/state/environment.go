// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/rtreego"
	"encoding/json"
	"encoding/gob"
	"io/ioutil"
	"bytes"
)

func init() {
	gob.Register(envImmutables{})
	gob.Register(EnvMutables{})
	gob.Register(Environment{})
}

// This variable represents the global up vector.
// Because Go doesn't support constant structures, this has to be a variable.
var GlobalUp geom.Vector = geom.Vector{0, 1, 0}

// envImmutables represents the immutable parts of an environment.
type envImmutables struct {
	meshes map[string]*Mesh	// This maps paths to meshes.
	paths map[uint]string	// This maps object ids to paths.
}

// MarshalBinary converts an envImmutables into a binary representation.
func (ei envImmutables) MarshalBinary() ([]byte, error) {
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the envImmutables' meshes and paths.
	if err := encoder.Encode(ei.meshes); err != nil {
		return nil, err
	}
	if err := encoder.Encode(ei.paths); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives an envImmutables from its binary representation.
func (ei *envImmutables) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Decode the envImmutables' meshes and paths.
	if err := decoder.Decode(&ei.meshes); err != nil {
		return err
	}
	if err := decoder.Decode(&ei.paths); err != nil {
		return err
	}
	
	return nil
}

// EnvMutables represents the mutable parts of an environment.
type EnvMutables struct {
	Objs *rtreego.Rtree	// This holds all the objects in the environment.
	Lights []Light		// This holds all the lights in the environment.
	Cam Camera			// This represents environment's camera.
}

// LinkTo creates a new environment by associating the mutable parts of an environment with the immutable parts of another environment.
// The EnvMutables em is modified in the process, and the returned environment uses em as its mutable part.
func (em *EnvMutables) LinkTo(e Environment) Environment {
	objs := em.Objs.SearchCondition(func(nbb *rtreego.Rect) bool {return true})
	
	for _, s := range objs {
		o := s.(*Object)
		
		// If the object's id and model path exist, update the object's mesh pointer.
		if path, exists := e.immutable.paths[o.id]; exists {
			if mesh, exists := e.immutable.meshes[path]; exists {
				o.mesh = mesh
			}else{
				o.mesh = nil
			}
		}else{
			o.mesh = nil
		}
	}
	
	// Because the mesh informs the object's bounds, we need to rebuild the tree.
	em.Objs = rtreego.NewTree(3, 2, 5, objs...)
	
	return Environment{
		immutable: e.immutable,
		mutable: em,
	}
}

// MarshalBinary converts an EnvMutables into a binary representation.
func (em EnvMutables) MarshalBinary() ([]byte, error) {
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the EnvMutables' objects, lights, and camera.
	if err := encoder.Encode(em.Objs.SearchCondition(func(nbb *rtreego.Rect) bool {return true})); err != nil {
		return nil, err
	}
	if err := encoder.Encode(em.Lights); err != nil {
		return nil, err
	}
	if err := encoder.Encode(em.Cam); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives an EnvMutables from its binary representation.
func (em *EnvMutables) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Decode the EnvMutables' objects, lights, and camera.
	var objects []rtreego.Spatial
	if err := decoder.Decode(&objects); err != nil {
		return err
	}
	if err := decoder.Decode(&em.Lights); err != nil {
		return err
	}
	if err := decoder.Decode(&em.Cam); err != nil {
		return err
	}
	
	// Rebuild an R-Tree for the objects.
	em.Objs = rtreego.NewTree(3, 2, 5)
	for _, s := range objects {
		o := s.(Object)
		em.Objs.Insert(&o)
	}
	
	return nil
}

// Environment represents a 3-dimensional space full of objects.
type Environment struct {
	immutable *envImmutables
	mutable *EnvMutables
}

// StoredEnvironment is used to (un)marshal environment data to/from the JSON format.
type StoredEnvironment struct {
	Objs []StoredObject		`json:"objs"`
	Lights []StoredLight	`json:"lights"`
	Cam StoredCamera		`json:"cam"`
}

// EnvironmentFromFile loads an environment from a JSON file.
func EnvironmentFromFile(path string) (Environment, error) {
	// Read in the JSON data from the file.
	inputBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return Environment{}, err
	}
	
	// Unmarshal the input data.
	var inputEnv StoredEnvironment
	err = json.Unmarshal(inputBytes, &inputEnv)
	if err != nil {
		return Environment{}, err
	}
	
	// Get the new environment ready.
	env := Environment{
		immutable: &envImmutables{
			meshes: make(map[string]*Mesh),
			paths: make(map[uint]string),
		},
		mutable: &EnvMutables{
			Objs: rtreego.NewTree(3, 2, 5),
			Lights: make([]Light, len(inputEnv.Lights), len(inputEnv.Lights)),
			Cam: Camera{},
		},
	}
	
	// Add objects to the environment.
	for i, inObj := range inputEnv.Objs {
		objMesh, exists := env.immutable.meshes[inObj.Model]
		
		if !exists {
			// If the new object's mesh has not already been loaded, load it.
			objMesh, err = MeshFromFile(relativePath(path, inObj.Model))
			if err != nil {
				// If we didn't find the mesh at the relative path, try the absolute path.
				objMesh, err = MeshFromFile(inObj.Model)
				if err != nil {
					return Environment{}, err
				}
			}
			
			// Add the mesh to the mesh map.
			env.immutable.meshes[inObj.Model] = objMesh
		}
		
		// Map the new object's id to the object's model path.
		env.immutable.paths[uint(i + 1)] = inObj.Model
		
		// Add the new object to the objects tree.
		env.mutable.Objs.Insert(&Object{
			Pos: inObj.Pos,
			id: uint(i + 1),
			mesh: objMesh,
		})
	}
	
	// Add lights to the environment.
	for i, inLight := range inputEnv.Lights {
		env.mutable.Lights[i] = Light{
			Pos: inLight.Pos,
			Col: colour.NewRGB(inLight.Col.R, inLight.Col.G, inLight.Col.B),
		}
	}
	
	// Add the camera to the environment.
	env.mutable.Cam, err = NewCamera(inputEnv.Cam.Pos, inputEnv.Cam.Dir, inputEnv.Cam.Fov)
	if err != nil {
		return Environment{}, err
	}
	
	return env, nil
}

// MarshalBinary converts the immutable parts of an environment into a binary representation.
// The mutable parts should be encoded separately and re-associated using LinkTo().
func (e Environment) MarshalBinary() ([]byte, error) {
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the environment's immutables.
	if err := encoder.Encode(*e.immutable); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives the immutable parts of an environment from its binary representation.
// The mutable parts should be decoded separately and re-associated using LinkTo().
func (e *Environment) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Set up the environment.
	e.immutable = new(envImmutables)
	e.mutable = nil
	
	// Decode the environment's immutables.
	if err := decoder.Decode(e.immutable); err != nil {
		return err
	}
	
	return nil
}

// Mutable returns a pointer to the mutable elements of an environment.
func (e Environment) Mutable() *EnvMutables {
	return e.mutable
}