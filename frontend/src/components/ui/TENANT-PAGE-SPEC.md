# Seitengerüst des Tenant-Portals — verbindlicher Umbau-Spec

Ziel: JEDE Seite unter `frontend/src/app/[tenant]/(protected)` hat dasselbe
Gerüst. Kein „ungefähr gleich", kein Sonderweg pro Seite. Es gibt KEINE
Ausnahmeseite mehr — Startseite, Profil und Notfallliste tragen dasselbe
Gerüst wie jede Liste.

Nicht anfassen: Eltern-Portal (`app/parents`, `components/parent`),
Schul-Portal (`app/school`, `components/school`, `components/class-day`),
Operator-Portal (`app/operator`, `components/operator`).

## Das Gerüst

Wurzel jeder Seite ist `TenantPage` aus `~/components/ui/tenant-page`:

```tsx
<TenantPage
  title="Kinder"
  stats={statusLine} // echte Zahlen, KEIN Erklärsatz
  statsLoading={isLoading} // rendert an der Stelle ein Skelett
  actions={<Button size="md">Kind anlegen</Button>}
  search={{ value: query, onChange: setQuery, placeholder: "Kind suchen…" }}
  filters={filterConfigs} // FilterConfig[] aus ui/page-header/types
  tabs={{ value, onChange, items }}
  error={error}
  loading={isLoading}
  empty={hasNothing ? { title: "…", description: "…" } : null}
  back // auf Unterseiten: mobiler Zurück-Knopf
>
  {inhalt}
</TenantPage>
```

Gerenderte Reihenfolge, fest, nicht verhandelbar:

1. Kopfkarte: Titel + Aktionen rechts, darunter Statuszeile, darunter Suche
   und Filter (auf den Planungsflächen steht dort die `PlanningContextBar`)
2. Reiter (horizontal, eine einzige Bauart)
3. Inhalt im 24-px-Rhythmus, jede Fläche darin aus dem Kit

## Regeln

1. **Keine Mini-Überschrift über dem Titel.** Kein `kicker`, kein Overline,
   kein blauer Kleintext. Wo man ist, sagen Brotkrumen und Seitenleiste.
2. **Kein Layout in der Seite.** Kein `max-w`, kein `mx-auto`, kein eigenes
   `p-*`/`px-*` auf der Wurzel, kein `<main>`, kein `space-y-*` auf der Wurzel,
   keine eigene `<h1>`. Alles davon liefert `TenantPage` bzw. die Shell.
3. **Statuszeile = echte Daten**, die die Seite ohnehin lädt:
   „116 Kinder · 107 zuhause · 9 krank", „29.07.2026 bis 27.08.2026 ·
   22 Betreuungstage". Für Wert-Label-Paare gibt es `TenantPageStats`.
   Kein Erklärsatz, keine Marketingzeile. Beim Laden `statsLoading`.
4. **Aktionen gehören in `actions`.** Keine eigene Button-Zeile unter oder über
   dem Kopf, keine Zeile, die nur einen Zähler, ein Select oder einen
   Export-Knopf trägt. Export und Kebab sind Aktionen.
5. **Suche und Filter gehören in `search`/`filters`.** Handgebaute Zeilen aus
   `Input` + `CustomSelect` werden zu `FilterConfig[]` (`type: "dropdown"`).
   Nur wenn ein Filter wirklich nicht abbildbar ist: `type: "custom"` mit
   `render`, und den aktiven Zustand über `activeFilters` melden.
6. **Zustände kommen aus dem Gerüst**: `error`, `loading`, `empty`. Kein
   `return null`, kein freier Spinner, kein „Wird geladen…"-Fließtext, kein
   Fehler-Alert, der nur auf einem Breakpoint sichtbar ist.
7. **Reiter**: Seitenreiter über `tabs`. `ui/Tabs` bleibt nur für Reiter
   INNERHALB einer Karte (Slide-over: Bearbeiten/Verlauf), `SegmentedControl`
   für jede Wertauswahl — auch Monat/Woche/Tag, A/B-Woche, Exportzeitraum,
   Listenfilter. Kein drittes Bauteil, und kein `ui/Tabs` als Umschalter für
   einen Wert.
8. **Karten** im Inhalt kommen aus dem Kit, nie aus einer eigenen
   Klassenkette:
   - `SectionCard` — Inhaltsfläche mit Kopf; OHNE `title` die reine Fläche
     (das frühere `<section className="moto-content-surface …">`).
   - `TileCard` — anklickbare Kachel (Kind, Raum, Mitarbeiter, Nachricht).
     Mit `containsControls`, wenn die Kachel eigene Bedienelemente trägt.
   - `StatCard` — jede Kennzahl, mit `icon`, `href` und `loading`.
   - `TenantPageHeaderSkeleton` / `CardSkeleton` — Ladezustände.
     Kein `rounded-3xl`, kein `rounded-xl` als Karte, keine handgerollte
     Kartenfläche und keine zweite Kennzahl-Kachel.
9. **Farben** nur über Kit-Komponenten oder `LOCATION_COLORS` /
   `moto-*`-Klassen. Keine generischen Tailwind-Hues für Marken-Semantik.
10. **Texte**: Sie-Form, echte Umlaute, keine Gender-Doppelpunkte, `…` statt
    `...`, „…" als Anführungszeichen. Bindestrich nie als Gedankenstrich.
11. **Verhalten bleibt gleich.** Diese Umstellung ist Layout, keine fachliche
    Änderung. Keine Berechtigungsprüfung, kein Datenabruf, keine Sortierung
    und keine Fehlerbehandlung inhaltlich ändern.
12. **Bestehende Tests nicht weichspülen.** Wenn ein Test bricht, weil Text
    umgezogen ist oder eine Klasse sich geändert hat: Selektor anpassen. Wenn
    ein Test wegen einer FACHLICHEN Erwartung bricht: stehen lassen und im
    Bericht melden, nichts umschreiben.
13. **Keine AI-Attribution** irgendwo (Code, Kommentare, Commits).

## Planungsflächen

Betreuungsplan, Dienstplan, Vertretung und Kalender haben keinen zweiten
Kopf: `PlanningContextBar` ist keine Karte mehr, sondern das Bedienband IN
der Kopfkarte (`TenantPage` Slot `searchSlot`) — dieselbe Stelle, an der eine
Liste ihre Suche trägt. Der Ansichtsumschalter darin ist ein
`SegmentedControl`, die Kontextzeile (Wochenleiste, Zeitraum, Lücken) bleibt
die zweite Zeile des Bandes.

## Wenn der Kopf in einer View-Komponente steckt

Manche Seiten (`planung`, `betreuungsplan`, `dateien`, `calendar`,
`students/[id]` …) rendern nur eine View-Komponente. Dann wandert das Gerüst in
diese Komponente, nicht in die Seite — Ergebnis muss dasselbe sein.
`PlanningContextBar` bleibt als Zeitnavigation bestehen, sitzt aber als Inhalt
UNTER der Kopfkarte, nicht als eigener Seitenkopf.

## Abnahme je Datei

```bash
cd frontend
npx prettier --write <deine Dateien> --log-level error
npx tsc --noEmit                 # höchstens zweimal, am Ende deines Blocks
npx vitest run <pfade deiner Seiten>   # nur die Tests deines Blocks
```

Fertig ist eine Seite, wenn `TenantPage` ihre Wurzel ist, keine der Regeln 1
bis 13 verletzt ist und tsc sowie ihre Tests grün sind.
