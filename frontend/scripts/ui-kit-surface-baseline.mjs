import baseline from "./ui-kit-surface-baseline.json" with { type: "json" };

// ui-kit/no-hand-rolled-surface und ui-kit/no-hand-rolled-page-card (#5594,
// scripts/oxlint-plugin-ui-kit.mjs).
//
// Funde je Datei, erzeugt mit `pnpm run ui-kit:surface-baseline`.
// Shrink-only: mehr Funde schlagen im Lint fehl, weniger lassen den Test
// „surface baseline“ fehlschlagen, bis die Zahl gesenkt ist. Die Teil-Issues
// von #5594 bauen den Bestand je Bereich ab; der letzte Teil-PR stellt beide
// Regeln auf hart null. Eine begründete Ausnahme steht als
// `oxlint-disable-next-line` am Fund, nicht in einer Allowlist.
/** @type {Record<string, Record<string, number>>} */
export const surfaceBaseline = baseline;
