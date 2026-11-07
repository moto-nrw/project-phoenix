import baseline from "./ui-kit-inset-baseline.json" with { type: "json" };

// ui-kit/no-hand-rolled-inset (#5592, scripts/oxlint-plugin-ui-kit.mjs).
//
// Funde je Datei, erzeugt mit `pnpm run ui-kit:inset-baseline`. Shrink-only:
// mehr Funde schlagen im Lint fehl, weniger lassen den Test „inset baseline“
// fehlschlagen, bis die Zahl gesenkt ist. Der letzte Teil-PR der Migration
// stellt die Regel auf hart null.
/** @type {Record<string, Record<string, number>>} */
export const insetBaseline = baseline;
