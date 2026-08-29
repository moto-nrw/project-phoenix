# Hilfebereich: Umbau- und Implementierungsplan

**Status:** Konzept abgeschlossen, klickbarer Prototyp in Arbeit
**Zielbranch:** `development`
**Issue:** #2229 (Hilfebereich neu strukturieren)
**Grundlagen:** ADR 0008 (Format und Plattform), ADR 0009 (Zielgruppen und Personalisierung)

## 1. Ausgangslage

Gemessen am 29.08.2026, Details in ADR 0008/0009. Das Nötigste:

- `guide-data.ts`: 3.001 Zeilen (264 KB), 23 Kapitel, 108 Schritte, vier Kapitelsätze.
- `/help/features` rendert 7 Kapitel mit ~60 Themen auf **einer** Route; das PDF hat 91 Seiten.
- 652 Commits auf die Datei, **36 davon Merge-Konflikte**; 88 % der Änderungen kommen im Feature-PR mit.
- Suchindex wird auf Modulebene gebaut → die komplette 264-KB-Datei liegt im Client-Bundle jeder Hilfeseite.
- `PageHeaderWithSearch` wird von 89 Dateien benutzt (74 Seiten unter `(protected)`).
- Anker-Ids sind bereits URL-sichere Slugs, abgesichert durch `guide-search.test.ts`.
- 77 Screenshots (3,4 MB) unter `public/help/screens/`.

## 2. Leitentscheidungen (bereits getroffen)

1. Typisiertes TS bleibt, kein MDX, kein Anbieter, kein zweiter Build (ADR 0008).
2. Hilfe bleibt öffentlich; personalisiert wird der **Weg dorthin**, nicht der Hilfebereich (ADR 0009).
3. Die Grenze der Inhaltsbestände folgt dem Portal, nicht der Rolle. Leitung und Betreuungskraft
   bekommen getrennte Einstiege auf einem gemeinsamen Bestand (ADR 0009).

### 2.1 Der Stack — es gibt kein Doku-Framework

**Der Hilfebereich ist eine Handvoll normaler Seiten in der bestehenden Next.js-App.** Es kommt keine
Doku-Software dazu, weder gehostet noch als Bibliothek: kein Mintlify, kein Docusaurus, kein
Starlight, kein Nextra, kein Fumadocs. Wer nach „welches Doku-Framework nutzen wir" sucht, findet
hier die Antwort: **keins, und das ist die Entscheidung** (ADR 0008).

Gebaut wird ausschließlich mit dem, was die App ohnehin mitbringt:

| Aufgabe | Womit | Wo |
|---|---|---|
| Seiten und Routing | Next.js 16 App Router, React 19 | `src/app/help/**` |
| Darstellung | eigene Komponenten aus dem moto-UI-Kit, Tailwind 4 | `src/components/help/`, `src/components/ui/` |
| Inhalte | typisiertes TS, `GuideChapter` → `GuideStep` | `src/components/help/guide-data*.ts` |
| Suche | fuse.js über einen aus den Inhalten abgeleiteten Index | `guide-search.ts`, `help-search.tsx` |
| Icons | lucide-react | in den Inhaltsdaten referenziert |
| Screenshots | `next/image` auf statischen Dateien | `public/help/screens/` (77 Dateien) |
| PDF | Playwright rendert die öffentlichen Seiten, pdf-lib legt den Hintergrund | `scripts/generate-guides.ts` |
| Tests | vitest | 5 Dateien, u. a. `help-search-sync.test.ts` |

Neue Abhängigkeiten sind für diesen Umbau **nicht vorgesehen**. Wird doch eine gebraucht, ist das
eine Abweichung von ADR 0008 und gehört in der PR-Beschreibung begründet.

### 2.2 Umfang von #2229: erst entscheiden, dann migrieren

#2229 liefert zunächst **keine vollständige Migration**. Das Issue endet mit einem klickbaren
Prototyp, der die neue Struktur an einem begrenzten Ausschnitt prüfbar macht. Die Arbeitspakete A
bis D beginnen erst nach der Teamentscheidung und werden als abgegrenzte Folgeaufgaben umgesetzt.

Der Prototyp beantwortet diese Frage:

> Welche Navigation hilft OGS-Personal, aus einem langen Kapitel schnell zu genau einem Thema zu
> kommen?

Er liegt isoliert unter `/help/prototype` und verändert die vier bestehenden Hilfe- und PDF-Routen
nicht. Der erste Vergleich umfasste drei deutlich verschiedene Varianten. Ausgewählt wurde die
Seitenleisten-Lösung: Auf Desktop bleibt die Navigation am linken Fensterrand stehen und scrollt
unabhängig vom Artikel. Sie ist keine Karte im Inhaltsbereich. Auf kleinen Bildschirmen wird sie zu
einer ausklappbaren Themenliste. Die verworfenen Aufgaben- und Sucheinstiege bleiben über die
Commit-Historie des Prototyp-Branches nachvollziehbar, sind aber nicht mehr Teil der Route.

Rolle, Thema und NFC-Variante sind teilbar. Beispiele:

```text
/help/prototype/kindersuche?role=caregiver
/help/prototype/datenverwaltung?role=lead
/help/prototype/nfc-kinder-auschecken?presence_mode=binary&schoolyard=enabled
```

`presence_mode=detailed|binary` bildet den Anwesenheitsmodus ab. Im Binär-Modus unterscheidet
`schoolyard=enabled|disabled` den Drei-Schaltflächen- vom Zwei-Schaltflächen-Ablauf. Der Prototyp
zeigt außerdem, wie ein alter Hash-Link im Browser auf eine Themenseite wechseln kann. Die spätere
Produktionslösung muss diese Weiterleitung so kapseln, dass Druck- und PDF-Ansichten unverändert
bleiben.

Ein kompakter Einstieg „Frag ChatGPT“ steht oben in der festen Seitenleiste und im mobilen Kopf.
Er öffnet ChatGPT mit einem vorausgefüllten Prompt. Der Prompt nennt die Adresse der aktuell
geöffneten Hilfeseite. Auf einer Themenseite nennt er zusätzlich das aktuelle Thema. Dieses Thema
dient nur als Startpunkt; Fragen zur gesamten moto-Hilfe bleiben ausdrücklich möglich. Auf der
Übersicht startet der Prompt ohne Thema. Damit lässt sich der Übergang ohne eigenen GPT und ohne
API-Anbindung erproben. Der Parameter `prompt` ist in der offiziellen OpenAI-Dokumentation nicht
beschrieben und bleibt deshalb vorerst eine Prototyp-Annahme.

Nur die Beispielseite „Ein Kind finden“ lässt sich zusätzlich in einem typischen
Dokumentationsstil öffnen. `article_style=docs` zeigt eine ruhige Artikeldarstellung mit klarer
Typografie, gegliederten Abschnitten und einer Navigation „Auf dieser Seite“. Die feste
Themenseitenleiste und die moto-Farben bleiben erhalten. Die normale Darstellung und alle anderen
Themen bleiben unverändert. Beispiel:

```text
/help/prototype/kindersuche?role=caregiver&article_style=docs
```

## 3. Arbeitspakete

### Strang P — klickbarer Prototyp für #2229

**P1 · Beispielstrecke schneiden** — Größe S
Das lange Kapitel „Alltag und Aufsicht“ wird beispielhaft in kurze, direkt verlinkbare Fragen
zerlegt. Einige Leitungs- und NFC-Themen zeigen zusätzlich die Rollen- und Moduslogik.

**P2 · Navigationsvarianten vergleichen** — Größe M, abgeschlossen
Drei Varianten wurden verglichen. Die feste Seitenleiste wurde ausgewählt; die beiden verworfenen
Varianten und der Entwicklungs-Schalter wurden danach aus der Route entfernt.

**P3 · Teilbare Zustände** — Größe S
Thema, Rolle, Anwesenheitsmodus und Schulhof-Schalter stehen in der URL. Rolle und Modus werden im
Hilfebereich nicht als Auswahl gezeigt: Die App setzt sie beim Einstieg. Alte Hash-Links werden im
Prototyp clientseitig auf die passende Themenseite übertragen.

**P4 · Responsive Prüfung** — Größe S
Die ausgewählte Seitenleisten-Lösung wird auf Desktop, Tablet und Smartphone geprüft. Navigation,
Direktlinks, Querverweise, der ChatGPT-Einstieg und die beispielhafte Dokumentationsdarstellung
müssen anklickbar sein. Es gibt keine schreibenden Aktionen.

**P4a · ChatGPT-Einstieg erproben** — Größe S
Ein kompakter Link oben prüft Platzierung, Beschriftung und den Wechsel zu einem externen Chat. Der
Prompt übergibt die aktuelle Hilfe-Adresse und, falls vorhanden, das Thema. ChatGPT muss die
öffentliche Hilfeseite selbst aufrufen können. Der Link überträgt keine Daten aus dem angemeldeten
Bereich.

**P4b · Dokumentationsstil an einer Seite erproben** — Größe S
Die Seite „Ein Kind finden“ erhält über `article_style=docs` eine zweite Artikeldarstellung nach dem
Muster typischer Dokumentationsseiten. Sie nutzt weiterhin die typisierten Prototyp-Daten. Die
anderen Themen und die normale Darstellung bleiben unverändert, bis das Team über die Übernahme
entscheidet.

**P5 · Entscheidung dokumentieren** — extern abhängig
Das Team bestätigt die ausgewählte Seitenleiste oder benennt nötige Anpassungen. Ergebnis und
Begründung kommen in #2229. Erst danach wird die Lösung produktionsreif neu umgesetzt.

### Arbeitspakete der anschließenden Migration

Vier Stränge nach der Prototypentscheidung. **A** und **B** sind unabhängig voneinander und können
parallel laufen; **D5** ist ein Defekt und hängt an nichts.

### Strang A — Gerüst (kein Inhaltswechsel)

**A1 · `guide-data.ts` zerlegen** — Größe M, Risiko **hoch (Koordination)**
Eine Datei pro Kapitel, Barrel-Export behält die heutigen Namen (`setupChapters`, `appChapters`,
`nfcChapters`, `nfcQuickstartChapters`), damit kein Konsument sich ändert. Rein mechanisch, kein Satz
Inhalt wird angefasst.
*Mit im selben PR:* `.claude/rules/help-guide-sync.md` und der Skill `help-guide-sync` benennen
`guide-data.ts` als „die Content-Datei" — beide werden sonst falsch.
*Fertig wenn:* `pnpm run check` grün, alle 5 Hilfe-Testdateien unverändert grün, Diff enthält keine
Textänderung.

**A2 · Route pro Thema** — Größe L, Risiko mittel
`/help/<topic>` (flach, siehe §7.1), generiert aus dem Kapitelbaum. Die Themen-Id bleibt die heutige
Anker-Id.
*Zwingend:* Weiterleitung der alten Anker-URLs (`/help/features#betreuungsplan` →
`/help/betreuungsplan`). Im Umlauf sind PDFs, Support-Mails und Suchtreffer.
*Fertig wenn:* jede der 108 Themen-Routen auflösbar, alte Anker leiten weiter, Suchtreffer zeigen auf
die neuen Routen.

**A3 · Seitenleiste und Querverweise** — Größe M, Risiko niedrig
Persistente Navigation aus dem Kapitelbaum, „verwandte Themen" am Seitenende. Bauteile aus
`components/ui/` (Regel `frontend-ui-kit.md`), kein fremdes Theme.

**A4 · Suchindex splitten** — Größe S, Risiko niedrig
Index pro Guide statt global auf Modulebene. Nebenwirkung: das 264-KB-Bundle verschwindet.
*Fertig wenn:* `help-search-sync.test.ts` grün, Bundle der Hilfeseiten messbar kleiner.

**A5 · PDF-Pipeline** — Größe S, Risiko niedrig
Durch den flachen Schnitt (§7.1) entfällt der befürchtete Umbau: die Themen liegen unter
`/help/<topic>`, die vier heutigen Guide-Routen (`/help/setup`, `/help/features`, `/help/nfc`,
`/help/nfc/erste-schritte`) bleiben unangetastet und **werden zur Druck- und PDF-Ansicht**. Keiner
dieser Slugs kollidiert mit einer der 108 Themen-Ids (geprüft). `generate-guides.ts` und der
CI-Schritt in `build.yml` ändern sich damit **gar nicht**. Kein PDF pro Thema.
*Fertig wenn:* die vier PDFs entstehen unverändert, `MIN_PDF_BYTES`-Prüfung greift.

### Strang B — Kontextuelle Hilfe (unabhängig von A)

**B1 · `HelpTopicId` ableiten** — Größe S, Risiko niedrig
Literal-Union aus dem Kapitelbaum. Ein Tippfehler wird damit zum Compile-Fehler.

**B2 · `helpTopic` im Seitenkopf** — Größe S, Risiko niedrig
Prop an `PageHeaderWithSearch`, gerendert als Icon-Button mit `aria-label="Hilfe zu dieser Seite"`.
Eine Komponente, 89 Aufrufstellen. Verlinkt **vor A2** auf `/help/features#<id>`, danach ohne
Änderung an den Aufrufstellen auf `/help/<id>`.
*Regeln:* kein Thema → kein Symbol (nie ersatzweise auf die Startseite); immer gleicher Tab als
normaler `<Link>` — ungespeicherte Eingaben schützt der vorhandene `use-navigation-guard` von selbst
(§7.2); mobil nicht mit dem globalen Hilfe-Eintrag der Bottom-Nav konkurrieren.

**B3 · Zwei Wächter** — Größe S, Risiko niedrig
(a) Auflösbarkeit: jede verwendete `HelpTopicId` ergibt eine existierende URL.
(b) Abdeckung, **shrink-only**: jede `page.tsx` unter `(protected)` setzt `helpTopic` oder steht auf
einer Ausnahmeliste, die nur kürzer werden darf — Muster wie `serialTestBaseline`.

**B4 · Seiten verdrahten** — Größe M, laufend
Schrittweise, beginnend bei den Seiten mit dem meisten Support-Aufkommen. Die Ausnahmeliste aus B3
schrumpft dabei.

**B5 · Klickzählung (optional)** — Größe S
Zähler pro Thema, ohne Personenbezug. Ergebnis ist eine Landkarte der unverständlichen Stellen und
damit die Datengrundlage für Strang D.

### Strang C — Neue Zielgruppen (eigene Issues, nicht #2229)

**C1 · Lehrkraft-Anleitung** — Größe S
Die drei vorhandenen Lehrkraft-Themen aus `appChapters` in eine eigene Anleitung, `school-nav-items.ts:86`
umhängen. Heute landet eine Lehrkraft auf der vollständigen OGS-Anleitung.

**C2 · Eltern-Hilfe** — Größe L
Existiert gar nicht. Zielt auf die belegten Fälle #2296 (AGs/Gruppen wird als Anmeldung gelesen) und
#2297 (Push ohne installierte App). Bestes Verhältnis von Aufwand zu belegtem Nutzen.

### Strang D — Inhalt (das eigentliche Volumen)

**D1 · Informationsarchitektur** — Größe L
Neuer Schnitt nach Aufgabe und Rolle statt nach Sidebar-Bereich. Einstiege „Ich möchte …" /
„Was mache ich, wenn …?".

**D2 · Getrennte Rollen-Einstiege** — Größe M
Zwei Einstiegskarten auf `/help` — „Für Betreuungskräfte" und „Für die Leitung" — je mit eigener
Startseite und eigener Themenreihenfolge, sodass es sich wie zwei Anleitungen liest. **Ein
Inhaltsbestand dahinter** (ADR 0009). Leitungsthemen stehen nicht zwischen den Themen der
Betreuungskraft, bleiben aber über „Weitere Themen für die Leitung" und über die Suche erreichbar —
die durchsucht immer den ganzen Bestand.
Zuordnung Kapitel → Rolle **neben der Kapiteldefinition**, nicht in einer separaten Liste (sonst
veraltet sie stumm).
*Fertig wenn:* eine Betreuungskraft findet DATEV, Datenverwaltung und Anmeldephasen nicht zwischen
ihren Themen — und findet sie trotzdem, wenn sie danach sucht.

**D3 · Inhalte neu schneiden** — Größe XL, laufend
Kürzen, teilen, entfernen. Screenshots teilweise neu aufnehmen. Texte nach
`moto-einfache-sprache`, Prüfung nach `verstaendlichkeit.md`.

**D4 · Prüfung mit OGS-Personal** — extern abhängig
Drei typische Abläufe (AC 4 in #2229). Terminabhängig, blockiert nichts anderes — deshalb früh
anstoßen, nicht früh einplanen.

**D5 · `presence_mode`-Verzweigung in der NFC-Anleitung** — Größe S, **Defekt**
Für Schulen im Binär-Modus beschreibt die Anleitung heute Raumauswahl, Raumwechsel und WC-Taste, die
es dort nicht gibt. Hängt an nichts und sollte vorgezogen werden.

## 4. Reihenfolge

```
für #2229:            P1 → P2 → P3 → P4 → P5
nach Teamfreigabe:    D5 (Defekt) · B1 → B2 → B3 → B4 (läuft weiter)
Gerüst der Migration: A1 → A2 → { A3, A4, A5 }
danach:               D1 → D2 → D3 (läuft weiter)
parallel, eigene Issues: C1 · C2
früh anstoßen:        D4
```

Begründung der Reihenfolge: **B liefert Nutzen ohne den großen Umbau** — die Anker-Ids existieren
heute schon, das Hilfesymbol funktioniert also vor A2 und überlebt A2 ohne Änderung an den
Aufrufstellen. **A1 ist der Flaschenhals**, nicht wegen des Aufwands, sondern wegen der Konflikte
(siehe Risiken). **D setzt A1/A2 voraus**, weil Inhalte sonst wieder in einer Datei landen.

## 5. Risiken

| Risiko | Warum | Umgang |
|---|---|---|
| **A1 kollidiert mit jedem offenen Feature-Branch** | 88 % der Guide-Änderungen laufen im Feature-PR mit; 36 Konflikte gab es schon ohne Umbau | In **einem** PR, angekündigt, zügig gemergt. Nicht über mehrere Tage offen halten. Ideal: an einem Tag mit wenigen offenen Guide-Branches |
| Alte Anker-URLs brechen | PDFs, Support-Mails, Suchtreffer im Umlauf | Weiterleitungen in A2 sind Pflicht, nicht Kür |
| ~~PDF-Erzeugung bricht~~ | entfällt: die vier Guide-Routen bleiben als Druckansicht bestehen (§7.1, A5) | `MIN_PDF_BYTES` bleibt der Wächter |
| Regelwerk veraltet | `help-guide-sync.md` und der gleichnamige Skill benennen `guide-data.ts` als die Content-Datei | Im selben PR wie A1 aktualisieren |
| Themen-Ids werden zum Vertrag | B2 bindet App-Seiten an Themen; ein Umbenennen bricht den Link | B3(a) macht es zum Compile-/Testfehler statt zum stillen Bruch |
| Screenshot-Drift | 77 Bilder, neuer Schnitt macht einen Teil ungültig | In D3 mitplanen, nicht nachgelagert |

## 6. Was wir NICHT bauen

- Keine mandantenspezifische Hilfe und kein PDF pro Schule (ADR 0009).
- Keine Filterung nach den ~86 Einstellungen — nur die vier Fähigkeits-Schalter, die in
  `sidebar.tsx` ohnehin schon die Navigation steuern.
- Keine harte Rollenfilterung innerhalb eines Portals; Vorbelegung ja, Ausblenden nein.
- Kein PDF pro Thema.
- Keine Authentifizierung vor dem Hilfebereich.

## 7. Entschiedene Detailfragen

**7.1 Themen-Routen sind flach** — `/help/betreuungsplan`, nicht `/help/features/betreuungsplan`.
Alle 108 Themen-Ids und 23 Kapitel-Ids sind bereits global eindeutig, ohne Überschneidung zwischen
beiden Mengen — ein flacher Namensraum kollidiert nirgends. Ausschlaggebend ist aber D1/D3: der neue
Schnitt **wird** Themen zwischen den Anleitungen verschieben. Verschachtelte URLs bräche jede solche
Verschiebung, und mit ihnen die Hilfe-Links aus der App (B2). Flach bleibt die Adresse stabil, egal
in welcher Anleitung ein Thema am Ende steht. Betrifft die Weiterleitungen in A2.

**7.2 Keine Sonderregel für Formularseiten in B2.** `src/lib/hooks/use-navigation-guard.ts`
existiert bereits und fängt In-App-`<Link>`-Klicks in der Capture-Phase ab, bevor der Next-Router
sie sieht; fünf Stellen nutzen ihn schon (Stammdaten-Tab, Anmeldeformular-Editor, Essensplan u. a.).
Der Hilfe-Link ist ein normaler `<Link>` im selben Tab und läuft damit automatisch in diesen Schutz.
Kein Flag pro Seite, keine zentrale Liste, kein `target="_blank"`.

**7.3 A1 läuft, sobald das Fenster frei ist** — gemessen am 29.08.2026 berührt genau **ein** offener
PR die Hilfe-Komponenten (#2767). Empfehlung: A1 direkt nach dessen Merge, in einem PR, an einem Tag.
Der Check vor dem Start:

```bash
gh pr list --state open --json number,title,files \
  --jq '.[] | select(.files[].path | startswith("frontend/src/components/help/")) | "\(.number) \(.title)"'
```

**7.4 Situations-Einstiege („Ich möchte …") kommen nach D3**, nicht in den ersten Wurf. Sie wären ein
dritter Weg in die Hilfe neben den Rollenkarten (D2) und den Hilfe-Symbolen in der App (B2). Gute
Situations-Einstiege lassen sich erst schreiben, wenn die Themen neu geschnitten sind — vorher legt
man eine aufgabenförmige Schicht über softwareförmige Inhalte und wiederholt damit genau die Kritik
aus #2229 eine Ebene höher.
