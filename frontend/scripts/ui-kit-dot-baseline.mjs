import baseline from "./ui-kit-dot-baseline.json" with { type: "json" };

// Punkt-Ratchet (#5598, scripts/oxlint-plugin-ui-kit.mjs).
//
// Baseline: Funde je Datei zum Stand von #5598, erzeugt mit
// `pnpm run ui-kit:dot-baseline`. Shrink-only: eine Datei mit mehr Funden
// schlägt im Lint fehl; eine mit weniger lässt den Test „dot ratchet baseline“
// fehlschlagen, bis die Zahl gesenkt ist. Was hier steht, liegt außerhalb von
// Legenden und Plänen und bekommt deshalb keinen StatusDot.
/** @type {Record<string, Record<string, number>>} */
export const dotRatchetBaseline = baseline;
