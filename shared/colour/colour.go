// Package colour provides shared a colour object for use by workers and the master.
package colour

import (
	"encoding/gob"
	"bytes"
	"math"
)

func init() {
	gob.Register(RGB{})
}

// RGB represents a colour with red, green, and blue channels.
// All channels are normalized so they're within the range [0, 1].
type RGB struct {
	r, g, b float64
}

// StoredRGB is used to (un)marshal colour data to/from the JSON format.
type StoredRGB struct {
	R uint8	`json:"r"`
	G uint8	`json:"g"`
	B uint8	`json:"b"`
}

// NewRGB returns a new RGB object with the specified colours.
func NewRGB(r, g, b uint8) RGB {
	return RGB{r: float64(r) / 255.0, g: float64(g) / 255.0, b: float64(b) / 255.0}
}

// NewRGBFromFloats returns a new RGB object with the specified colours (after clamping them to the range [0, 1]).
func NewRGBFromFloats(r, g, b float32) RGB {
	return RGB{r: math.Max(0.0, math.Min(float64(r), 1.0)), g: math.Max(0.0, math.Min(float64(g), 1.0)), b: math.Max(0.0, math.Min(float64(b), 1.0))}
}

// Add returns the sum of the RGB objects a and b.
func (a RGB) Add(b RGB) RGB {
	return RGB{r: math.Min(a.r + b.r, 1.0), g: math.Min(a.g + b.g, 1.0), b: math.Min(a.b + b.b, 1.0)}
}

// Scale returns the RGB object a scaled by the scalar s.
func (a RGB) Scale(s float64) RGB {
	return RGB{r: math.Max(0.0, math.Min(s * a.r, 1.0)), g: math.Max(0.0, math.Min(s * a.g, 1.0)), b: math.Max(0.0, math.Min(s * a.b, 1.0))}
}

// Multiply returns the product of the RGB objects a and b.
func (a RGB) Multiply(b RGB) RGB {
	return RGB{r: a.r * b.r, g: a.g * b.g, b: a.b * b.b}
}

// RGBA returns the three colour channels of an RGB object in the range [0, 255], and 0 for the alpha channel.
// This function allows RGB objects to be used with the Color (image/color) interface.
func (rgb RGB) RGBA() (uint32, uint32, uint32, uint32) {
	return uint32(255 * rgb.r), uint32(255 * rgb.g), uint32(255 * rgb.b), uint32(0)
}

// RGB returns the three colour channels of an RGB object in the range [0, 255].
func (rgb RGB) RGB() (uint8, uint8, uint8) {
	return uint8(255 * rgb.r), uint8(255 * rgb.g), uint8(255 * rgb.b)
}

// MarshalBinary converts an RGB colour into a binary representation.
func (rgb RGB) MarshalBinary() ([]byte, error) {
	r, g, b := rgb.RGB()
	
	// Set up the binary encoder.
	writer := bytes.Buffer{}
	encoder := gob.NewEncoder(&writer)
	
	// Encode the colour's r, g, and b values.
	if err := encoder.Encode(r); err != nil {
		return nil, err
	}
	if err := encoder.Encode(g); err != nil {
		return nil, err
	}
	if err := encoder.Encode(b); err != nil {
		return nil, err
	}
	
	return writer.Bytes(), nil
}

// UnmarshalBinary derives an RGB value from its binary representation.
func (rgb *RGB) UnmarshalBinary(data []byte) error {
	// Set up the binary decoder.
	reader := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(reader)
	
	// Decode the colour's r, g, and b values.
	var r, g, b uint8
	if err := decoder.Decode(&r); err != nil {
		return err
	}
	if err := decoder.Decode(&g); err != nil {
		return err
	}
	if err := decoder.Decode(&b); err != nil {
		return err
	}
	
	// Reconstruct the colour.
	*rgb = NewRGB(r, g, b)
	
	return nil
}