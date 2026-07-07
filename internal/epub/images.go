package epub

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // register decoder
	"image/jpeg"
	_ "image/png" // register decoder
	"path"
	"strings"
)

// Image-size guards: anything decodable whose long edge exceeds maxEdgePass or
// whose byte size exceeds maxBytesPass is downscaled to targetEdge and
// re-encoded as JPEG. EPUB illustrations are usually already web-sized, so most
// images pass through verbatim; the guard exists for photo-heavy books that
// would otherwise bloat the archive.
const (
	maxEdgePass  = 1400
	maxBytesPass = 300 << 10 // 300 KB
	targetEdge   = 1200
	jpegQuality  = 80
)

// registerImage stores an image (by its source zip entry name) into the book's
// image map, processing it once; repeated references reuse the same output
// entry. Returns the output entry name ("images/imgN.ext"), or "" when the
// source entry is missing/unreadable.
func (p *parser) registerImage(srcEntry string) string {
	if name, ok := p.imgNameBySrc[srcEntry]; ok {
		return name
	}
	data, err := readNamed(p.files, srcEntry)
	if err != nil || len(data) == 0 {
		p.imgNameBySrc[srcEntry] = ""
		return ""
	}
	processed, ext := processImage(data, strings.ToLower(path.Ext(srcEntry)))
	p.imgCount++
	name := fmt.Sprintf("images/img%d%s", p.imgCount, ext)
	if p.book.Images == nil {
		p.book.Images = map[string][]byte{}
	}
	p.book.Images[name] = processed
	p.imgNameBySrc[srcEntry] = name
	return name
}

// processImage passes small images through verbatim and downscales/re-encodes
// oversized ones (long edge → targetEdge, JPEG). Undecodable bytes pass through
// with their original extension.
func processImage(data []byte, origExt string) ([]byte, string) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if origExt == "" {
			origExt = ".img"
		}
		return data, origExt
	}
	ext := extForFormat(format, origExt)
	long := max(cfg.Width, cfg.Height)
	if long <= maxEdgePass && len(data) <= maxBytesPass {
		return data, ext
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, ext
	}
	if long > targetEdge {
		scale := float64(targetEdge) / float64(long)
		w := max(1, int(float64(cfg.Width)*scale+0.5))
		h := max(1, int(float64(cfg.Height)*scale+0.5))
		img = scaleBilinear(img, w, h)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return data, ext
	}
	if buf.Len() >= len(data) {
		return data, ext
	}
	return buf.Bytes(), ".jpg"
}

func extForFormat(format, origExt string) string {
	switch format {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	}
	if origExt != "" {
		return origExt
	}
	return ".img"
}

// scaleBilinear resamples src to w×h. A dependency-free bilinear filter — it
// runs on a handful of oversized illustrations per book, so clarity beats speed.
func scaleBilinear(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		fy := (float64(y) + 0.5) * float64(sh) / float64(h)
		y0 := int(fy - 0.5)
		ty := fy - 0.5 - float64(y0)
		y1 := min(y0+1, sh-1)
		if y0 < 0 {
			y0 = 0
		}
		for x := 0; x < w; x++ {
			fx := (float64(x) + 0.5) * float64(sw) / float64(w)
			x0 := int(fx - 0.5)
			tx := fx - 0.5 - float64(x0)
			x1 := min(x0+1, sw-1)
			if x0 < 0 {
				x0 = 0
			}
			c00 := rgbaAt(src, b.Min.X+x0, b.Min.Y+y0)
			c10 := rgbaAt(src, b.Min.X+x1, b.Min.Y+y0)
			c01 := rgbaAt(src, b.Min.X+x0, b.Min.Y+y1)
			c11 := rgbaAt(src, b.Min.X+x1, b.Min.Y+y1)
			var out [4]uint8
			for i := 0; i < 4; i++ {
				top := float64(c00[i])*(1-tx) + float64(c10[i])*tx
				bot := float64(c01[i])*(1-tx) + float64(c11[i])*tx
				out[i] = uint8(top*(1-ty) + bot*ty + 0.5)
			}
			dst.SetRGBA(x, y, color.RGBA{out[0], out[1], out[2], out[3]})
		}
	}
	return dst
}

func rgbaAt(img image.Image, x, y int) [4]uint8 {
	r, g, b, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}
