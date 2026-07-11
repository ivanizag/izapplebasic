package izapplebasic

import (
	"image"

	"github.com/ivanizag/izapple2/screen"
)

// Snapshot renders the emulated screen with the pixels widened to
// compensate the vertical stretch of the CRT line separation, as
// izapple2 does when saving to a file.
func (env *Environment) Snapshot() image.Image {
	img := screen.Snapshot(env.mem, screen.ScreenModeNTSC)
	return widePixelsFilter(img)
}

// VideoModeName describes the active video mode, like "TEXT40COL"
// or "HGR-MIX40".
func (env *Environment) VideoModeName() string {
	return screen.VideoModeName(env.mem)
}

func widePixelsFilter(in *image.RGBA) *image.RGBA {
	bounds := in.Bounds()
	factor := 1200 / bounds.Dx()
	if factor < 2 {
		return in
	}
	out := image.NewRGBA(image.Rect(0, 0, factor*bounds.Dx(), bounds.Dy()))
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			c := in.At(x, y)
			for i := 0; i < factor; i++ {
				out.Set(factor*x+i, y, c)
			}
		}
	}
	return out
}
