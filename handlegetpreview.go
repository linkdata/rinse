package rinse

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"

	"golang.org/x/image/draw"
)

func (rns *Rinse) handleGetPreview(w http.ResponseWriter, r *http.Request) {
	if job := rns.findJob(r.PathValue("uuid")); job != nil {
		switch job.State() {
		case JobNew:
			w.WriteHeader(http.StatusAccepted)
		case JobFailed:
			w.WriteHeader(http.StatusNoContent)
			return
		default:

			job.mu.Lock()
			var pageNames []string
			maxPage := len(job.imgfiles)
			for fn := range job.imgfiles {
				pageNames = append(pageNames, fn)
			}
			job.mu.Unlock()
			sort.Strings(pageNames)

			if maxPage == 0 {
				w.WriteHeader(http.StatusAccepted)
				return
			}

			if s := r.URL.Query().Get("pages"); s != "" {
				if n, e := strconv.Atoi(s); e == nil {
					if n > 0 {
						maxPage = min(maxPage, n)
					}
				}
			}

			imgWidth := 172
			if s := r.URL.Query().Get("width"); s != "" {
				if n, e := strconv.Atoi(s); e == nil {
					if n > 0 && n <= 1920 {
						imgWidth = n
					}
				}
			}

			var images []image.Image
			var heights []int

			fullrect := image.Rectangle{
				Max: image.Point{
					X: imgWidth,
					Y: 0,
				},
			}

			for page := 0; page < maxPage; page++ {
				if f, e := os.Open(path.Join(job.Workdir, pageNames[page])); e == nil {
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
			err := jpeg.Encode(&buf, dst, nil)
			if err == nil {
				hdr := w.Header()
				hdr["Content-Length"] = []string{strconv.Itoa(buf.Len())}
				hdr["Content-Type"] = []string{"image/jpeg"}
				w.Write(buf.Bytes())
				return
			}

			slog.Error("handleGetPreview", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}
