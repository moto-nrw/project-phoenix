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

type pathPoint struct{ x, y float32 }

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
	var current pathPoint
	var cmd string
	for index := 0; index < len(tokens); {
		token := tokens[index]
		var err error
		switch token {
		case "M", "c":
			cmd = token
			index++
			continue
		case "z":
			r.ClosePath()
			index++
			continue
		}
		current, index, err = tracePathSegment(r, tokens, index, cmd, current, project)
		if err != nil {
			return err
		}
	}
	return nil
}

func tracePathSegment(r *vector.Rasterizer, tokens []string, index int, command string, current pathPoint, project func(x, y float32) (float32, float32)) (pathPoint, int, error) {
	switch command {
	case "M":
		return traceMove(r, tokens, index, project)
	case "c":
		return traceCurve(r, tokens, index, current, project)
	default:
		return current, index, fmt.Errorf("avatar path: number %q before any command", tokens[index])
	}
}

func traceMove(r *vector.Rasterizer, tokens []string, index int, project func(x, y float32) (float32, float32)) (pathPoint, int, error) {
	coordinates, next, err := traceCoordinates(tokens, index, 2, "M")
	if err != nil {
		return pathPoint{}, index, err
	}
	current := pathPoint{x: coordinates[0], y: coordinates[1]}
	r.MoveTo(project(current.x, current.y))
	return current, next, nil
}

func traceCurve(r *vector.Rasterizer, tokens []string, index int, current pathPoint, project func(x, y float32) (float32, float32)) (pathPoint, int, error) {
	delta, next, err := traceCoordinates(tokens, index, 6, "c")
	if err != nil {
		return current, index, err
	}
	x1, y1 := project(current.x+delta[0], current.y+delta[1])
	x2, y2 := project(current.x+delta[2], current.y+delta[3])
	current.x += delta[4]
	current.y += delta[5]
	x3, y3 := project(current.x, current.y)
	r.CubeTo(x1, y1, x2, y2, x3, y3)
	return current, next, nil
}

func traceCoordinates(tokens []string, index, count int, command string) ([]float32, int, error) {
	if index+count > len(tokens) {
		return nil, index, fmt.Errorf("avatar path: incomplete %s at token %d", command, index)
	}
	coordinates := make([]float32, count)
	for i := range coordinates {
		value, err := strconv.ParseFloat(tokens[index+i], 32)
		if err != nil {
			return nil, index, fmt.Errorf("avatar path token %d %q: %w", index+i, tokens[index+i], err)
		}
		coordinates[i] = float32(value)
	}
	return coordinates, index + count, nil
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 0xFF} }
