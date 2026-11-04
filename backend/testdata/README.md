# Image regression fixture

`interlaced-logo.png` is a synthetic 32 × 16 RGBA PNG encoded with Adam7
interlacing (IHDR interlace method 1). Teal pixels alternate between full and
partial opacity. It contains no metadata or user data. Go's PNG decoder reads
it; gopdf cannot draw its original bytes. Tests cover legacy logos, upload
normalization, letter PDFs and record image fields (#5236).

`compatible-logo.png` is a synthetic 2 × 2 non-interlaced 8-bit PNG with an
ancillary text chunk. Its original bytes must survive the PDF fast path.
`interlaced-small-logo.png` is a minimal 1 × 1 Adam7 PNG that grows when Go
re-encodes it. It verifies the upload's size fallback and subsequent rendering.
The near-1-MiB optimized compatible PNG is generated deterministically in the
Organization & Tenancy domain test instead of storing a large binary fixture.
