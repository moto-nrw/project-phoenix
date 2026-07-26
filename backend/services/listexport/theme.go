package listexport

// Design tokens for rendered exports. These MIRROR the frontend design
// system and must stay in sync with:
//   - frontend/src/lib/location-helper.ts (LOCATION_COLORS)
//   - frontend/src/styles/globals.css (guide print block)
//
// Nothing enforces that sync today — when a token here changes, check the
// frontend sources above (and vice versa).

type rgb struct{ R, G, B uint8 }

var (
	// Brand.
	colorEyebrow = rgb{0x3F, 0x6F, 0x12} // guide cover eyebrow (dark green)
	colorInk     = rgb{0x17, 0x23, 0x1F} // guide near-black title

	// Neutrals (Tailwind gray scale, as used by xlsx.go + the guide).
	colorPageBg  = rgb{0xF9, 0xFA, 0xFB} // gray-50
	colorSurface = rgb{0xFF, 0xFF, 0xFF}
	colorBorder  = rgb{0xE5, 0xE7, 0xEB} // gray-200 — card border
	colorMuted   = rgb{0x6B, 0x72, 0x80} // gray-500
	colorBody    = rgb{0x11, 0x18, 0x27} // gray-900 — DataTable cell text

	// Table chrome, mirroring frontend/src/components/ui/data-table.tsx:
	// the header is NOT a filled band — it is muted gray-500 labels over a
	// gray-100 hairline; rows separate with gray-50. No dark fill anywhere.
	colorHeaderText = rgb{0x6B, 0x72, 0x80} // gray-500  (thead text)
	colorHeaderRule = rgb{0xF3, 0xF4, 0xF6} // gray-100  (thead border-b)
	colorRowLine    = rgb{0xF9, 0xFA, 0xFB} // gray-50   (row border-b)

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
	// Group heading = InfoCard's title (text-base font-semibold, 16px).
	// px -> pt is 0.75.
	fontGroup  = 12.0
	fontFooter = 7.0
)

// Layout (pt). A4 landscape.
const (
	pageMargin = 30.0
	cardRadius = 12.0
	// Card inner padding — InfoCard is p-4/sm:p-6 (16/24px). 18pt = 24px.
	cardPadX = 18.0
	cardPadY = 14.0
	// Gap under the group heading — InfoCard's mb-4 (16px).
	groupGap   = 12.0
	pillRadius = 7.0
	pillRowH   = 18.0 // vertical advance between wrapped filter-pill rows
	pillPadX   = 7.0  // horizontal inset of a pill's text
	pillGap    = 6.0  // horizontal gap between two pills

	// minBodyHeight is the body area the header must always leave usable.
	// Filter metadata is caller-supplied and unbounded (the care-usage
	// export joins every care-offering name into one label), so without a
	// floor here enough wrapped pill rows push bodyTop() past bodyBottom():
	// the record renderer then emits pages whose cards sit outside the media
	// box, and the table renderer fails on invalid rectangle coordinates.
	// 120pt holds a card with padding, a group heading, the table header,
	// and a body row.
	minBodyHeight = 120.0

	// Header block geometry. drawHeader() advances by exactly these
	// amounts and headerBottom() sums the same ones for the parts that
	// are actually drawn, so the body starts under the real header
	// instead of under a worst-case guess (a title without an eyebrow,
	// or an export without filters, used to leave the reserve unused).
	// TestDesignedPDFHeaderReserveMatchesDrawnHeader pins the two together.
	headerFirstY          = 32.0 // first text row, below the logo
	headerEyebrowAdvance  = 16.0
	headerTitleAdvance    = 14.0
	headerSubtitleAdvance = 14.0
	headerBodyGap         = 10.0 // breathing room between header and card

	cellPadX     = 6.0
	cellPadY     = 5.0
	rowLineHt    = 10.0
	logoHeight   = 20.0
	logoAspect   = 480.0 / 168.0 // assets/moto-logo.png pixel ratio
	footerHeight = 26.0
)
