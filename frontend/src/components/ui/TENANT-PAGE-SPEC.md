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
3. Inhalt im 24-px-Rhythmus, jede Fläche darin aus dem Kit; er füllt die
   Höhe des Bildschirms — die letzte Fläche wächst bis zur Unterkante

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
   **Höchstens drei Paare** — `TenantPageStats` schneidet dahinter ab. Die
   Statuszeile ist eine Orientierung; was mehr sagen will, ist eine Fläche
   im Inhalt, keine längere Zeile.
4. **Aktionen gehören in `actions`, und dort steht GENAU EINE sichtbar.**
   Keine eigene Button-Zeile unter oder über dem Kopf, keine Zeile, die nur
   einen Zähler, ein Select oder einen Export-Knopf trägt. Neben dem Titel
   steht die Aktion, die man täglich braucht („Neu", „Drucken",
   „Schichtarten verwalten"), daneben das Kebab-Menü mit allem Weiteren.
   Export gehört ins Menü, unter der Überschrift „Herunterladen" bzw.
   „Exportieren" — nie als Knopfreihe je Format. Kontextbedienelemente
   (Zeitraum, Datum) zählen nicht als Aktion und dürfen daneben stehen.
   Unter sm: eine Aktion steht neben der Statuszeile; ab zwei bleiben die
   Symbolknöpfe dort und jeder Textknopf bekommt eine volle Zeile darunter
   (Abschnitt „Unter sm").
5. **Suche und Filter gehören in `search`/`filters`.** Handgebaute Zeilen aus
   `Input` + `CustomSelect` werden zu `FilterConfig[]` (`type: "dropdown"`).
   Nur wenn ein Filter wirklich nicht abbildbar ist: `type: "custom"` mit
   `render`, und den aktiven Zustand über `activeFilters` melden. Ab etwa
   drei Filtern sammelt `filterVariant="quiet"` sie hinter einer
   Schaltfläche; darunter stehen sie in der Zeile. Beides sind Einstellungen
   des Gerüsts — keine Seite konfiguriert `PageHeaderWithSearch` selbst.
   Ein Filter mit Mehrfachauswahl (`type: "buttons"`, `multiSelect`) rendert
   als Chip-Reihe, ein Einfachfilter als `SegmentedControl`: eine
   Filterreihe darf nie aussehen wie eine zweite Reiterleiste.
6. **Zustände kommen aus dem Gerüst**: `error`, `loading`, `empty`. Kein
   `return null`, kein freier Spinner, kein „Wird geladen…"-Fließtext, kein
   Fehler-Alert, der nur auf einem Breakpoint sichtbar ist. Alle drei stehen
   auf einer Fläche, so wie der Inhalt, den sie ersetzen. Fehlende Rechte
   sind ein Zustand, kein Fehler: `ForbiddenPage` (grau, Schloss, ein Satz
   dazu, wer freischaltet), nicht ein roter Alarmkasten.
   Ein Leerzustand ohne Aktion und ohne Symbol ist EIN Satz in der Karte
   (das Gerüst rendert ihn so); die inszenierte Form mit Überschrift gibt
   es nur für einen nächsten Schritt (`action`) oder einen benannten
   Zustand mit Symbol (ForbiddenPage, FeatureOffPage). Eine Bühne für einen
   Sachverhalt, an dem niemand etwas tun kann, ist eine Aufgabe, die keine
   ist.
   **Der Rumpf füllt die Höhe.** Die Karte mit dem Leersatz, die Tabelle mit
   einem Eintrag und die Liste mit dreißig haben denselben Umriss: die
   letzte Fläche wächst bis zur Unterkante des Bildschirms
   (`.moto-tenant-body` in `globals.css`, die Shell reicht dafür eine
   Flex-Spalte bis zur Inhaltshülle durch). Vorher endete eine leere Seite
   nach einer 60-px-Zeile, darunter der halbe Bildschirm nacktes
   Punktraster. Es wächst nur eine Fläche aus dem Kit (`section` oder
   `div` mit `moto-content-surface`) direkt im Rumpf oder bis zu zwei
   Hüllen tiefer (Seitenhülle um Kennzahlen und Tabelle, `DataTable` um
   ihre Fläche) — jede Hülle dazwischen muss selbst eine Flex-Spalte sein
   (`flex flex-col`, bei `hidden lg:block` also `hidden lg:flex
   lg:flex-col`), sonst wächst nur die Hülle und die Fläche bleibt kurz.
   Gitter und Anklickbares (`TileCard`) wachsen nie. Eine Seite, deren
   letzter Block nicht wächst, verpackt ihn zu tief — nicht mit `min-h-*`
   nachhelfen, die Hülle abbauen.
7. **Reiter**: Seitenreiter über `tabs`. `ui/Tabs` bleibt nur für Reiter
   INNERHALB einer Karte (Slide-over: Bearbeiten/Verlauf), `SegmentedControl`
   für jede Wertauswahl — auch Monat/Woche/Tag, A/B-Woche, Exportzeitraum,
   Listenfilter. Kein drittes Bauteil, und kein `ui/Tabs` als Umschalter für
   einen Wert. Auf dem Telefon bleibt das Band ein Band: es scrollt
   waagerecht und zeigt jeden Reiter. Keine Auswahlliste — die zeigte nur
   den aktiven Wert („Status", „Betrieb") und las sich als Filter.
8. **Kopfkarte mobil**: keine seiteneigenen Umbruch-Wrapper um `actions`
   (kein `w-full flex-col sm:flex-row`). Ob die Aktionen neben dem Titel
   stehen oder rechtsbündig in eine eigene Zeile brechen, entscheidet die
   Kopfkarte für alle Seiten gleich. Die Statuszeile nennt keinen Zeitraum,
   den ein Bedienband (`PlanningContextBar`) direkt darunter schon trägt.
9. **Karten** im Inhalt kommen aus dem Kit, nie aus einer eigenen
   Klassenkette:
   - `SectionCard` — Inhaltsfläche mit Kopf; OHNE `title` die reine Fläche
     (das frühere `<section className="moto-content-surface …">`).
   - `TileCard` — anklickbare Kachel (Kind, Raum, Mitarbeiter, Nachricht).
     Mit `containsControls`, wenn die Kachel eigene Bedienelemente trägt.
   - `StatCard` — jede Kennzahl, mit `icon`, `href` und `loading`.
   - `CollectionGrid` — DAS Kachelgitter jeder Sammlung: Spalten aus
     `auto-fit` mit Mindest-Kachelbreite, Abstand 16 px (mobil 12 px).
     Keine Seite deklariert mehr ihr eigenes `grid grid-cols-* gap-6`.
   - `TenantPageHeaderSkeleton` / `CardSkeleton` — Ladezustände.
     Kein `rounded-3xl`, kein `rounded-xl` als Karte, keine handgerollte
     Kartenfläche und keine zweite Kennzahl-Kachel.
10. **Farben** nur über Kit-Komponenten oder `LOCATION_COLORS` /
    `moto-*`-Klassen. Keine generischen Tailwind-Hues für Marken-Semantik.
    Eine Bedeutung, eine Farbe, quer über alle Listen: grün ist anwesend,
    grau ist nicht da, rot ist krank oder ein Fehler, lila ist genehmigt
    abwesend, orange braucht Aufmerksamkeit ohne Fehler zu sein. Statuspillen
    sind hell mit farbigem Punkt (`StatusDotBadge`/`StatusBadge`), nie
    vollflächig in der Signalfarbe: zwanzig davon nebeneinander sind eine
    Wand, kein Status.

11. **Kacheln einer Liste sehen überall gleich aus.** Ein Name in einer
    Zeile (Vor- und Nachname zusammen, gekürzt statt umgebrochen),
    Kit-Innenabstand (`p-4 sm:p-5`), keine Deko beim Überfahren (kein
    Vergrößern, keine Glanzkante) und keine Hinweiszeile „Tippen für mehr
    Infos" — wohin die Kachel führt, sagt ihr `aria-label`. In Tabellen
    steht „Nachname, Vorname" (alphabetisch sortierbar), auf Kacheln
    „Vorname Nachname".
12. **Texte**: Sie-Form, echte Umlaute, keine Gender-Doppelpunkte, `…` statt
    `...`, „…" als Anführungszeichen. Bindestrich nie als Gedankenstrich.
13. **Verhalten bleibt gleich.** Diese Umstellung ist Layout, keine fachliche
    Änderung. Keine Berechtigungsprüfung, kein Datenabruf, keine Sortierung
    und keine Fehlerbehandlung inhaltlich ändern.
14. **Bestehende Tests nicht weichspülen.** Wenn ein Test bricht, weil Text
    umgezogen ist oder eine Klasse sich geändert hat: Selektor anpassen. Wenn
    ein Test wegen einer FACHLICHEN Erwartung bricht: stehen lassen und im
    Bericht melden, nichts umschreiben.
15. **Keine AI-Attribution** irgendwo (Code, Kommentare, Commits).
16. **Typo-Boden.** Nichts Lesbares unter 12 px, und `text-xs` (12 px) ist
    Versalien-Labels vorbehalten (KICKER, Kennzahl-Label, Badge-Zahl).
    Werte, Sätze und Bedienelemente sind `text-sm` oder größer; die
    wichtigste Aussage einer Fläche ist ihre größte (`text-2xl` für die
    eine Antwort, `text-sm leading-6` für den Satz darunter — das
    Eltern-Portal-Muster). `text-[9px]`…`text-[11px]` sind per Ratchet
    gesperrt (`ui-kit/no-tiny-text`, Baseline shrink-only).
17. **Eine Bedienhöhe: 40 px, unter sm 44 px.** `CONTROL_HEIGHT` im Gerüst
    zieht Knöpfe, Felder und Auswahllisten der Kopfkarte auf 40 px
    (Telefon: 44 px, die Touch-Untergrenze — dieselbe Begründung wie
    `Button size="touch"` der Eltern-App). `SegmentedControl` (Spur 40 px),
    `ToggleChip` (`h-10`) und die Navigationsgruppe der
    `PlanningContextBar` teilen dieses Maß. Kein 32-px-Bedienelement mehr
    neben einem 40er in derselben Zeile.
18. **Höchstens EIN Bedienband in der Kopfkarte.** Titel und Statuszeile
    sind Text; darunter steht GENAU EINE interaktive Zeile (Suche + Filter
    + eine Aktion, oder das Planungsband). Die Kontextzeile der
    `PlanningContextBar` ist die benannte Ausnahme für Werkzeugflächen
    (Wochenleiste) — in `text-sm`, nie als Kleingedrucktes, und nie als
    dritte Zeile mit weiteren Bedienelementen.

## Unter `sm`: dieselbe Reihenfolge, andere Fläche

Auf dem Telefon gilt dasselbe Gerüst mit derselben Reihenfolge, aber ohne
die Kosten einer Desktop-Karte in einer 390-px-Spalte. Das entscheidet
`TenantPage` und das Kit, nie die Seite:

- **Kein Kopf als Karte.** Statuszeile, Aktionen, Reiter und Suche stehen
  flach unter der Kopfzeile der Shell (kein Rahmen, kein Schatten, kein
  Innenrand); der Zurück-Knopf steht mit darin. Der Seitenname steht mobil nur in der Kopfzeile;
  das `h1` bleibt für Screenreader (`sr-only`). Unterseiten mit `back` und
  Objektseiten mit `leading` behalten ihren Titel, weil die Kopfzeile dort
  nur den Bereich nennt.
- **Aktionen**: eine Aktion steht neben der Statuszeile. Ab zwei Aktionen
  bleiben Symbolknöpfe (`data-icon-only`: Kebab, `Button size="icon"`,
  schwebender Anlegen-Knopf) neben der Statuszeile, jeder Textknopf bekommt
  darunter eine volle Zeile. Keine halb gefüllte Zeile, kein gedrungener
  Knopf neben Leerraum.
- **Reiter als Pillen** im scrollenden Band: aktiv gefüllt, der Rest grau.
  Die Grundlinie des Desktops liest sich mobil als Text.
- **Dichte**: Blöcke im 12-px-Rhythmus (`space-y-3`), `SectionCard` mit
  `p-4` und `rounded-xl`, `StatCard` ohne Symbol mit `p-3`, Kacheln
  (`TileCard`, Kinderkarte) mit `p-3`. Der 24-px-Rhythmus ist ein
  Desktop-Maß.

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
bis 14 verletzt ist und tsc sowie ihre Tests grün sind.
