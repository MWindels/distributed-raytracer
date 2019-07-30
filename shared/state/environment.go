// Package state provides shared state information for use by workers and the master.
package state

import (
	"github.com/mwindels/distributed-raytracer/shared/geom"
	"github.com/mwindels/distributed-raytracer/shared/colour"
	"github.com/mwindels/rtreego"
	"encoding/json"
	"io/ioutil"
)

// This variable represents the global up vector.
// Because Go doesn't support constant structures, this has to be a variable.
var GlobalUp geom.Vector = geom.Vector{0, 1, 0}

// Environment represents a 3-dimensional space full of objects.
type Environment struct {
	Objs *rtreego.Rtree
	Lights []Light
	Cam Camera
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
	
	// Create new objects.
	var objs []rtreego.Spatial
	for _, inObj := range inputEnv.Objs {
		// Read in the current object.
		outObj, err := ObjectFromFile(relativePath(path, inObj.Model))
		if err != nil {
			// If we can't find the object at the relative path, try the absolute path.
			outObj, err = ObjectFromFile(inObj.Model)
			if err != nil {
				return Environment{}, err
			}
		}
		
		// Update the current object's position.
		outObj.Pos = inObj.Pos
		
		// Append the current object to the list of objects.
		objs = append(objs, outObj)
	}
	
	// Create new lights.
	var lights []Light
	for _, inLight := range inputEnv.Lights {
		// Create a new light.
		outLight := Light{
			Pos: inLight.Pos,
			Col: colour.NewRGB(inLight.Col.R, inLight.Col.G, inLight.Col.B),
		}
		
		// Append the current light to the list of lights.
		lights = append(lights, outLight)
	}
	
	// Create a new camera.
	cam, err := NewCamera(inputEnv.Cam.Pos, inputEnv.Cam.Dir, inputEnv.Cam.Fov)
	if err != nil {
		return Environment{}, err
	}
	
	// Create a new environment from the data.
	env := Environment{
		Objs: rtreego.NewTree(3, 2, 5, objs...),
		Lights: lights,
		Cam: cam,
	}
	
	return env, nil
}