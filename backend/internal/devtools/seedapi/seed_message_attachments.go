package api

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
)

// Photos and files in the seeded conversations (#6685). Without them every
// chat of a development machine shows text only, and the part a school uses
// most, a quick photo of the day, can be looked at nowhere.

// seedMessageFile is one file a seeded message carries.
type seedMessageFile struct {
	name    string
	content []byte
}

// uploadMessageFiles uploads the files under the session and returns the
// attachment_ids the message names.
func uploadMessageFiles(rt *Runtime, auth AuthRef, path string, files ...seedMessageFile) ([]string, error) {
	ids := make([]string, 0, len(files))
	for _, file := range files {
		raw, err := rt.Client.PostFileWithAuthFields(auth, path, "file", file.name, file.content, nil)
		if err != nil {
			return nil, fmt.Errorf("upload message attachment %q: %w", file.name, err)
		}
		id, err := parseEnvelopeStringID(raw)
		if err != nil {
			return nil, fmt.Errorf("parse message attachment %q: %w", file.name, err)
		}
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	return ids, nil
}

// seedLanternPhoto is the photo of the finished lanterns: the flyer picture
// of the Laternenfest in the size of a phone photo.
func seedLanternPhoto() seedMessageFile {
	const scale = 8
	lanterns := []struct{ x, y, r float64 }{{48, 54, 13}, {86, 62, 15}, {124, 52, 12}}
	return seedPhoto("Laternen fertig.png", 168*scale, 112*scale, func(x, y float64) color.NRGBA {
		return lanternPixel(x/scale, y/scale, 168, 112, lanterns)
	})
}

// seedMeadowPhoto is a photo of the afternoon outside: sun, sky, two hills
// and a kite.
func seedMeadowPhoto(name string) seedMessageFile {
	const width, height = 1200, 900
	return seedPhoto(name, width, height, func(x, y float64) color.NRGBA {
		t := y / height
		c := color.NRGBA{R: uint8(120 + 90*t), G: uint8(185 + 45*t), B: uint8(245 - 15*t), A: 255}
		if d := math.Hypot(x-930, y-190); d < 150 {
			c = blend(c, color.NRGBA{R: 255, G: 226, B: 120, A: 255}, math.Min(1, (150-d)/60))
		}
		if d := math.Abs(x-360+0.6*(y-250)) + math.Abs(y-250)*0.8; d < 46 {
			c = color.NRGBA{R: 232, G: 84, B: 72, A: 255}
		}
		if y > 560+70*math.Sin(x/190) {
			c = color.NRGBA{R: 126, G: 190, B: 78, A: 255}
		}
		if y > 660-55*math.Sin(x/260+1.3) {
			c = color.NRGBA{R: 86, G: 158, B: 64, A: 255}
		}
		return c
	})
}

func seedPhoto(name string, width, height int, pixel func(x, y float64) color.NRGBA) seedMessageFile {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetNRGBA(x, y, pixel(float64(x), float64(y)))
		}
	}
	var out bytes.Buffer
	// Encoding into a buffer cannot fail for an in-memory NRGBA image.
	_ = png.Encode(&out, img)
	return seedMessageFile{name: name, content: out.Bytes()}
}

// seedPickupSlip is the signed slip a parent photographs or scans.
func seedPickupSlip() seedMessageFile {
	return seedMessageFile{name: "Abholerlaubnis Zoo.pdf", content: demoPDF("Abholerlaubnis für den Zoo-Ausflug", []string{
		"Mein Kind darf nach dem Ausflug von der Großmutter abgeholt werden.",
		"Die Abholung ist um 15:30 Uhr am Schultor.",
		"Unterschrift der Sorgeberechtigten liegt vor.",
	})}
}

// seedCraftList is the material list the OGS sends to a group.
func seedCraftList() seedMessageFile {
	return seedMessageFile{name: "Bastelliste Laternen.pdf", content: demoIllustratedPDF("Das brauchen wir zum Basteln", lanternPicture(), []string{
		"Leere Marmeladengläser, gespült",
		"Transparentpapier in bunten Farben",
		"Alte Kleidung, die schmutzig werden darf",
	})}
}
