// ui-kit/no-important-override (#5581, scripts/oxlint-plugin-ui-kit.mjs).
//
// Funde je Datei zum Stand von #5581. Input und OverflowMenu hängen className
// per Zeichenkette an statt über cn(); dort gewinnt ohne `!` die Kit-Klasse.
// Die Stellen gehen weg, sobald die Felder die passende Größe als Prop
// bekommen. Shrink-only: mehr Funde schlagen im Lint fehl, weniger lassen den
// Test „no-important-override baseline is current“ fehlschlagen, bis die
// Zahl hier gesenkt ist.
/** @type {Readonly<Record<string, number>>} */
export const importantOverrideBaseline = {
  "src/app/[tenant]/(protected)/meal-plan/page.tsx": 1,
  "src/components/help/help-view.tsx": 1,
  "src/components/staff/dienstplan-person-week-grid.tsx": 1,
};
