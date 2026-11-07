import baseline from "./ui-kit-uppercase-label-baseline.json" with { type: "json" };

// ui-kit/no-uppercase-label (#5590, scripts/oxlint-plugin-ui-kit.mjs).
//
// Funde je Datei, erzeugt mit `pnpm run ui-kit:uppercase-label-baseline`.
// Shrink-only: mehr Funde schlagen im Lint fehl, weniger lassen den Test
// „uppercase label baseline“ fehlschlagen, bis die Zahl gesenkt ist. Der letzte
// Teil-PR der Migration stellt die Regel auf hart null.
/** @type {Record<string, Record<string, number>>} */
export const uppercaseLabelBaseline = baseline;
