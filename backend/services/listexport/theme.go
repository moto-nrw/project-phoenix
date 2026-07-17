package listexport

// Design tokens for rendered exports. These MIRROR the frontend design
// system and must stay in sync with:
//   - frontend/src/lib/location-helper.ts (LOCATION_COLORS)
//   - frontend/src/styles/globals.css (guide print block)
//
// Nothing enforces that sync today — see theme_sync_test.go.

type rgb struct{ R, G, B uint8 }

var (
	// Brand.
	colorBrandGreen = rgb{0x83, 0xCD, 0x2D} // LOCATION_COLORS.GROUP_ROOM
	colorEyebrow    = rgb{0x3F, 0x6F, 0x12} // guide cover eyebrow (dark green)
	colorInk        = rgb{0x17, 0x23, 0x1F} // guide near-black title

	// Neutrals (Tailwind gray scale, as used by xlsx.go + the guide).
	colorPageBg     = rgb{0xF9, 0xFA, 0xFB} // gray-50
	colorSurface    = rgb{0xFF, 0xFF, 0xFF}
	colorBorder     = rgb{0xE5, 0xE7, 0xEB} // gray-200
	colorRowLine    = rgb{0xF3, 0xF4, 0xF6} // gray-100
	colorGroupBg    = rgb{0xF3, 0xF4, 0xF6} // gray-100
	colorHeaderBg   = rgb{0x1F, 0x28, 0x37} // gray-800 — same as xlsx.go header
	colorHeaderText = rgb{0xFF, 0xFF, 0xFF}
	colorMuted      = rgb{0x6B, 0x72, 0x80} // gray-500

	// The guide's dotted background is #111827 at 12% over #F9FAFB. PDF
	// alpha would need an ExtGState; the blend is constant here, so it is
	// pre-multiplied to an opaque tone instead.
	colorDot = rgb{0xDD, 0xDF, 0xE2}
)

// Dot grid geometry, mirroring applyGuidePdfBackground in
// frontend/scripts/generate-guides.ts.
const (
	dotSpacingPt = 10.5
	dotRadiusPt  = 0.75
	dotOffsetPt  = 0.75
)

// Type scale (pt).
const (
	fontTitle    = 18.0
	fontSubtitle = 9.5
	fontEyebrow  = 7.5
	fontMeta     = 8.0
	fontTableHd  = 8.0
	fontBody     = 8.5
	fontGroup    = 9.5
	fontFooter   = 7.0
)

// Layout (pt). A4 landscape.
const (
	pageMargin   = 30.0
	cardRadius   = 12.0
	pillRadius   = 7.0
	cellPadX     = 6.0
	cellPadY     = 5.0
	rowLineHt    = 10.0
	logoHeight   = 20.0
	logoAspect   = 3.0 // moto-logo-mit-schriftzug.png is ~1200x400
	footerHeight = 26.0
)
