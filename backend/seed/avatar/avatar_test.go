package avatar

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"testing"
)

func TestPNGIsDeterministic(t *testing.T) {
	t.Parallel()
	first, err := PNGVariant(Index("Mia Wagner"), 128)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PNGVariant(Index("Mia Wagner"), 128)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same name rendered two different pictures")
	}
}

func TestIndexIgnoresCaseAndOuterSpace(t *testing.T) {
	t.Parallel()
	if Index("Mia Wagner") != Index("  mia wagner ") {
		t.Fatal("case or surrounding whitespace changed the picture")
	}
}

func TestIndexReachesEveryPoseDirectionAndTint(t *testing.T) {
	t.Parallel()
	poseSeen := make(map[int]bool)
	mirrorSeen := make(map[bool]bool)
	tintSeen := make(map[Tint]bool)
	for i := range 200 {
		c := choiceAt(Index(fmt.Sprintf("Kind %d", i)))
		poseSeen[c.Pose] = true
		mirrorSeen[c.Mirrored] = true
		tintSeen[c.Tint] = true
	}
	if len(poseSeen) != len(poses) || len(mirrorSeen) != 2 || len(tintSeen) != len(tints) {
		t.Fatalf("200 names reached %d/%d poses, %d/2 directions, %d/%d tints",
			len(poseSeen), len(poses), len(mirrorSeen), len(tintSeen), len(tints))
	}
}

func TestRenderDrawsFigureOnTint(t *testing.T) {
	t.Parallel()
	for poseIndex := range poses {
		name := nameForPose(t, poseIndex)
		data, err := PNGVariant(Index(name), 256)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("pose %d: not a PNG: %v", poseIndex, err)
		}
		if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
			t.Fatalf("pose %d: size %v, want 256x256", poseIndex, b)
		}
		tint := choiceAt(Index(name)).Tint
		// Top corners stay background: the crop leaves room beside the arms.
		if got := img.At(0, 0); !sameColor(got, tint.Background) {
			t.Errorf("pose %d: corner %v, want background %v", poseIndex, got, tint.Background)
		}
		figure := 0
		for y := range 256 {
			for x := range 256 {
				if sameColor(img.At(x, y), tint.Figure) {
					figure++
				}
			}
		}
		// The cropped figure covers a large part of the frame, well
		// above stray anti-aliasing and well below a flooded square.
		if share := float64(figure) / (256 * 256); share < 0.15 || share > 0.7 {
			t.Errorf("pose %d: figure covers %.0f%% of the picture", poseIndex, share*100)
		}
	}
}

func TestRenderRejectsNonPositiveSize(t *testing.T) {
	t.Parallel()
	if _, err := PNGVariant(Index("Mia Wagner"), 0); err == nil {
		t.Fatal("size 0 was accepted")
	}
}

func nameForPose(t *testing.T, pose int) string {
	t.Helper()
	for i := range 100 {
		name := fmt.Sprintf("Person %d", i)
		if choiceAt(Index(name)).Pose == pose {
			return name
		}
	}
	t.Fatalf("no name found for pose %d", pose)
	return ""
}

func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func TestDistinctGivesEveryPersonOwnPicture(t *testing.T) {
	t.Parallel()
	names := make([]string, Variants())
	for i := range names {
		names[i] = fmt.Sprintf("Person %d", i)
	}
	variants := Distinct(names)
	seen := make(map[int]bool)
	for i, v := range variants {
		if seen[v] {
			t.Fatalf("%s got variant %d a second time", names[i], v)
		}
		seen[v] = true
	}
	if variants[0] != Index(names[0]) {
		t.Fatalf("first name moved from its own variant %d to %d", Index(names[0]), variants[0])
	}
}

func TestDistinctKeepsOwnVariantWithoutCollision(t *testing.T) {
	t.Parallel()
	a, b := "", ""
	for i := 0; a == "" || b == ""; i++ {
		name := fmt.Sprintf("Kind %d", i)
		switch {
		case a == "":
			a = name
		case Index(name) != Index(a):
			b = name
		}
	}
	got := Distinct([]string{a, b})
	if got[0] != Index(a) || got[1] != Index(b) {
		t.Fatalf("names without collision changed variants: got %v, want [%d %d]", got, Index(a), Index(b))
	}
	twin := Distinct([]string{a, a})
	if twin[0] == twin[1] {
		t.Fatalf("two people with the same name share variant %d", twin[0])
	}
}

func TestPNGVariantRejectsUnknownVariant(t *testing.T) {
	t.Parallel()
	if _, err := PNGVariant(Variants(), 64); err == nil {
		t.Fatal("variant outside the range was accepted")
	}
}
