import baseline from "./ui-kit-pill-baseline.json" with { type: "json" };

// Pillen-Ratchet (#5584, scripts/oxlint-plugin-ui-kit.mjs).
//
// Baseline: Funde je Datei und Regel zum Stand von #5584, erzeugt mit
// `pnpm run ui-kit:pill-baseline`. Shrink-only: eine Datei mit mehr Funden
// schlägt im Lint fehl; eine mit weniger lässt den Test „pill ratchet
// baseline“ fehlschlagen, bis die Zahl gesenkt ist. Die Teil-Issues von #5584
// bauen sie je Bereich ab.
/** @type {Record<string, Record<string, number>>} */
export const pillRatchetBaseline = baseline;
