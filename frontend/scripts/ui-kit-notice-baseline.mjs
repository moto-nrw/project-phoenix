import baseline from "./ui-kit-notice-baseline.json" with { type: "json" };

// ui-kit/no-hand-rolled-notice (#5591, scripts/oxlint-plugin-ui-kit.mjs).
//
// Funde je Datei, erzeugt mit `pnpm run ui-kit:notice-baseline`.
// Shrink-only: mehr Funde schlagen im Lint fehl, weniger lassen den Test
// „notice baseline“ fehlschlagen, bis die Zahl gesenkt ist. Der letzte
// Teil-PR der Migration stellt die Regel auf hart null.
/** @type {Record<string, Record<string, number>>} */
export const noticeBaseline = baseline;

/** @type {{ rule: string; file: string; lines: number[]; reason: string }[]} */
export const noticeAllowlist = [];
