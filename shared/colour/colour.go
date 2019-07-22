// Package colour provides shared a colour object for use by workers and the master.
package colour

import "math"

// RGB represents a colour with red, green, and blue channels.
// All channels are normalized so they're within the range [0, 1].
type RGB struct {
	r, g, b float64
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