# Image regression fixture

`interlaced-logo.png` is a synthetic 32 × 16 RGBA PNG encoded with Adam7
interlacing (IHDR interlace method 1). Teal pixels alternate between full and
partial opacity. It contains no metadata or user data. Go's PNG decoder reads
it; gopdf cannot draw its original bytes. Tests cover legacy logos, upload
normalization, letter PDFs and record image fields (#5236).
