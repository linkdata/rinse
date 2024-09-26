package rinser

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path"
	"sort"

	"golang.org/x/image/draw"
)

func (job *Job) Preview(numPages, imgWidth int) (b []byte, err error) {
	var pageNames []string

	job.mu.Lock()
	numPages = max(1, numPages)
	numPages = min(len(job.imgfiles), numPages)
	imgWidth = max(96, imgWidth)
	imgWidth = min(1920, imgWidth)
	key := uint64(numPages)<<16 | uint64(imgWidth)
	if numPages > 0 {
		if b = job.previews[key]; b == nil {
			for fn := range job.imgfiles {
				pageNames = append(pageNames, fn)
			}
			sort.Strings(pageNames)
		}
	}
	job.mu.Unlock()

	if b == nil {
		var images []image.Image
		var heights []int

		fullrect := image.Rectangle{
			Max: image.Point{
				X: imgWidth,
				Y: 0,
			},
		}

		for page := 0; err == nil && page < numPages; page++ {
			var f *os.File
			if f, err = os.Open(path.Join(job.Datadir, pageNames[page])); err == nil {
				if src, e := png.Decode(f); e == nil {
					images = append(images, src)
					factor := float64(imgWidth) / float64(src.Bounds().Dx())
					height := int(float64(src.Bounds().Dy()) * factor)
					heights = append(heights, height)
					fullrect.Max.Y += height
				}
				f.Close()
			}
		}

		if err == nil {
			dst := image.NewRGBA(fullrect)
			y := 0
			for i, src := range images {
				rect := fullrect
				rect.Min.Y = y
				y += heights[i]
				rect.Max.Y = y
				draw.BiLinear.Scale(dst, rect, src, src.Bounds(), draw.Over, nil)
			}

			var buf bytes.Buffer
			if err = jpeg.Encode(&buf, dst, nil); err == nil {
				b = buf.Bytes()
				job.mu.Lock()
				job.previews[key] = b
				job.mu.Unlock()
			}
		}
	}

	return
}
