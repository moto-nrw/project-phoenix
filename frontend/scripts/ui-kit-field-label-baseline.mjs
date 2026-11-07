import baseline from "./ui-kit-field-label-baseline.json" with { type: "json" };

// ui-kit/no-hand-rolled-field-label (#5589, scripts/oxlint-plugin-ui-kit.mjs).
//
// Funde je Datei, erzeugt mit `pnpm run ui-kit:field-label-baseline`.
// Shrink-only: mehr Funde schlagen im Lint fehl, weniger lassen den Test
// „field label baseline“ fehlschlagen, bis die Zahl gesenkt ist. Der letzte
// Teil-PR der Migration stellt die Regel auf hart null.
/** @type {Record<string, Record<string, number>>} */
export const fieldLabelBaseline = baseline;
