// Package avatar draws profile pictures from a person's name.
//
// Every picture shows one figure of the moto logo, cut off so that head and
// raised arms fill the frame, on a light brand tint. The name alone decides
// figure, facing direction and color, so the same person always gets the
// same picture and no randomness or storage is involved.
package avatar

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"

	"golang.org/x/image/vector"
)

// DefaultSize is the edge length in pixels that seed uploads use.
const DefaultSize = 512

type figure struct {
	width, height float32
	path          string
}

// crop is the square of a figure, in figure pixels, that ends up in the
// picture. It sits on head and arms, so the body leaves the frame at the
// bottom.
type crop struct{ x, y, size float32 }

type pose struct {
	figure *figure
	crop   crop
}

// Tint pairs a logo color with its soft background. Values mirror
// MOTO_COLOR_PALETTE in frontend/src/lib/location-helper.ts. The logo red
// is left out on purpose: in the app red means "krank".
type Tint struct {
	Figure     color.RGBA
	Background color.RGBA
}

var (
	tints = []Tint{
		{Figure: rgb(0x83, 0xCD, 0x2D), Background: rgb(0xEE, 0xF9, 0xE1)}, // green
		{Figure: rgb(0x50, 0x80, 0xD8), Background: rgb(0xED, 0xF3, 0xFC)}, // blue
		{Figure: rgb(0xF7, 0x8C, 0x10), Background: rgb(0xFF, 0xF3, 0xE5)}, // orange
	}
	poses = []pose{
		{figure: &figureAdult, crop: crop{x: 40, y: -20, size: 480}},
		{figure: &figureChild, crop: crop{x: -5, y: -15, size: 370}},
	}
)

// Choice is what a variant draws: figure, direction and color.
type Choice struct {
	Pose     int // 0 adult, 1 child
	Mirrored bool
	Tint     Tint
}

// Variants is the number of distinct pictures: every figure facing both
// ways in every color.
func Variants() int { return len(poses) * 2 * len(tints) }

// Index returns the variant in [0, Variants()) a name resolves to. Case and
// surrounding whitespace do not change the result.
func Index(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return int(h.Sum32() % uint32(Variants()))
}

func choiceAt(variant int) Choice {
	return Choice{
		Pose:     variant % len(poses),
		Mirrored: (variant/len(poses))%2 == 1,
		Tint:     tints[(variant/(2*len(poses)))%len(tints)],
	}
}

// Distinct assigns variants to people shown together, such as the children
// and staff of one school. Each name keeps its own variant unless an earlier
// name already took it; then it moves to the next free one, so no two people
// share a picture while there are enough variants. The result depends only
// on the names and their order.
func Distinct(names []string) []int {
	variants := make([]int, len(names))
	taken := make(map[int]bool, Variants())
	for i, name := range names {
		if len(taken) == Variants() {
			clear(taken) // more people than pictures: start a new round
		}
		v := Index(name)
		for taken[v] {
			v = (v + 1) % Variants()
		}
		taken[v] = true
		variants[i] = v
	}
	return variants
}

// PNGVariant renders variant (see Variants) as a square PNG of size pixels.
func PNGVariant(variant, size int) ([]byte, error) {
	if variant < 0 || variant >= Variants() {
		return nil, fmt.Errorf("avatar variant %d outside [0, %d)", variant, Variants())
	}
	img, err := render(choiceAt(variant), size)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode avatar: %w", err)
	}
	return buf.Bytes(), nil
}

func render(choice Choice, size int) (*image.RGBA, error) {
	if size <= 0 {
		return nil, fmt.Errorf("avatar size must be positive, got %d", size)
	}
	p := poses[choice.Pose]

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.NewUniform(choice.Tint.Background), image.Point{}, draw.Src)

	scale := float32(size) / p.crop.size
	project := func(x, y float32) (float32, float32) {
		// potrace units are tenths of a pixel with y pointing up.
		fx, fy := x/10, p.figure.height-y/10
		if choice.Mirrored {
			fx = p.figure.width - fx
		}
		return (fx - p.crop.x) * scale, (fy - p.crop.y) * scale
	}

	r := vector.NewRasterizer(size, size)
	if err := tracePath(r, p.figure.path, project); err != nil {
		return nil, err
	}
	r.Draw(img, img.Bounds(), image.NewUniform(choice.Tint.Figure), image.Point{})
	return img, nil
}

// tracePath feeds potrace output into the rasterizer: absolute M, relative
// cubic c (repeated segments may omit the letter) and z.
func tracePath(r *vector.Rasterizer, path string, project func(x, y float32) (float32, float32)) error {
	// potrace glues commands to their first number ("M1224", "c-143").
	tokens := strings.Fields(strings.NewReplacer("M", " M ", "c", " c ", "z", " z ").Replace(path))
	var cx, cy float32
	var cmd string
	num := func(i int) (float32, error) {
		v, err := strconv.ParseFloat(tokens[i], 32)
		if err != nil {
			return 0, fmt.Errorf("avatar path token %d %q: %w", i, tokens[i], err)
		}
		return float32(v), nil
	}
	for i := 0; i < len(tokens); {
		tok := tokens[i]
		switch tok {
		case "M", "c":
			cmd = tok
			i++
			continue
		case "z":
			r.ClosePath()
			i++
			continue
		}
		switch cmd {
		case "M":
			if i+1 >= len(tokens) {
				return fmt.Errorf("avatar path: incomplete M at token %d", i)
			}
			x, err := num(i)
			if err != nil {
				return err
			}
			y, err := num(i + 1)
			if err != nil {
				return err
			}
			cx, cy = x, y
			r.MoveTo(project(cx, cy))
			i += 2
		case "c":
			if i+5 >= len(tokens) {
				return fmt.Errorf("avatar path: incomplete c at token %d", i)
			}
			var d [6]float32
			for k := range d {
				v, err := num(i + k)
				if err != nil {
					return err
				}
				d[k] = v
			}
			x1, y1 := project(cx+d[0], cy+d[1])
			x2, y2 := project(cx+d[2], cy+d[3])
			cx, cy = cx+d[4], cy+d[5]
			x3, y3 := project(cx, cy)
			r.CubeTo(x1, y1, x2, y2, x3, y3)
			i += 6
		default:
			return fmt.Errorf("avatar path: number %q before any command", tok)
		}
	}
	return nil
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 0xFF} }
