# Hilfebereich: Umbau- und Implementierungsplan

**Status:** Umgestellt. Der Prototyp ist die Hilfe; `/help` zeigt ihn, das
alte Guide-System ist entfernt. Was davon noch offen ist, steht in Abschnitt 8.
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

| Aufgabe            | Womit                                                                    | Wo                                           |
| ------------------ | ------------------------------------------------------------------------ | -------------------------------------------- |
| Seiten und Routing | Next.js 16 App Router, React 19                                          | `src/app/help/**`                            |
| Darstellung        | eigene Komponenten aus dem moto-UI-Kit, Tailwind 4                       | `src/components/help/`, `src/components/ui/` |
| Inhalte            | typisiertes TS, ein flacher `HelpTopic` je Frage                         | `src/components/help/help-content.ts`        |
| Suche              | Filter über Titel, Frage, Kurztext, Schritte und Tipps, nur im Browser   | in `help-view.tsx`                           |
| Icons              | lucide-react                                                             | in den Inhaltsdaten referenziert             |
| Screenshots        | `next/image`; die Artikel setzen derzeit kein `image`                    | `public/help/screens/` (87 Dateien)          |
| PDF                | Playwright rendert nur noch den gedruckten NFC-Onepager                  | `scripts/generate-guides.ts`                 |
| Tests              | vitest                                                                   | u. a. `help-topics.test.ts` als Wächter      |

Neue Abhängigkeiten sind für diesen Umbau **nicht vorgesehen**. Wird doch eine gebraucht, ist das
eine Abweichung von ADR 0008 und gehört in der PR-Beschreibung begründet.

### 2.2 Umfang von #2229: erst entscheiden, dann migrieren

#2229 liefert zunächst **keine vollständige Migration**. Das Issue endet mit einem klickbaren
Prototyp, der die neue Struktur an einem begrenzten Ausschnitt prüfbar macht. Die Arbeitspakete A
bis D beginnen erst nach der Teamentscheidung und werden als abgegrenzte Folgeaufgaben umgesetzt.

Der Prototyp beantwortet diese Frage:

> Welche Navigation hilft OGS-Personal, aus einem langen Kapitel schnell zu genau einem Thema zu
> kommen?

Er lag zunächst isoliert unter `/help/prototype`. Seit der Umstellung ist er die Hilfe: er liegt
unter `/help`, die drei Guide-Seiten sind entfernt und leiten dorthin weiter. Der erste Vergleich
umfasste drei deutlich verschiedene Varianten. Ausgewählt wurde die
Seitenleisten-Lösung: Auf Desktop bleibt die Navigation am linken Fensterrand stehen und scrollt
unabhängig vom Artikel. Sie ist keine Karte im Inhaltsbereich. Auf kleinen Bildschirmen wird sie zu
einer ausklappbaren Themenliste. Die verworfenen Aufgaben- und Sucheinstiege bleiben über die
Commit-Historie des Prototyp-Branches nachvollziehbar, sind aber nicht mehr Teil der Route.

Rolle, Thema und die drei anleitungsrelevanten Konfigurationswerte sind teilbar. Beispiele:

```text
/help/kindersuche?role=caregiver&nfc_enabled=false&presence_mode=binary&group_mode=open_care
/help/datenverwaltung?role=lead&nfc_enabled=true&presence_mode=detailed&group_mode=fixed_groups
/help/nfc-kinder-auschecken?nfc_enabled=true&presence_mode=detailed&group_mode=fixed_groups
```

Die Parameter bilden ausschließlich die drei in
`docs/research/help-guide-configuration-impact.md` priorisierten Faktoren ab:

- `nfc_enabled=true|false` für `attendance.nfc_enabled`,
- `presence_mode=detailed|binary` für `operations.presence_mode`,
- `group_mode=fixed_groups|open_care` für `operations.group_mode`.

Die App setzt diese Werte beim Einstieg aus den aufgelösten Mandanten-Metadaten. Im Hilfebereich
gibt es dafür keine Auswahl. Die Werte sind nur Kontext für passende Hinweise, Schritte und Bilder;
sie sind weder Berechtigungsprüfung noch harte Themenfilterung. Interne Themenlinks, Querverweise und
Suchtreffer erhalten alle drei Parameter. Fehlt ein Wert oder ist er ungültig, bleibt die öffentliche
Hilfe erreichbar und zeigt keine nur aus diesem Wert abgeleitete Variante. `schoolyard` gehört
ausdrücklich nicht zu diesem URL-Vertrag.

Alte Hash-Links wechseln im Browser auf die Themenseite. Das trägt jetzt die Weiterleitung mit:
`/help/features#kindersuche` landet als `/help#kindersuche`, weil ein Browser das Fragment über
eine Weiterleitung ohne eigenes Fragment mitnimmt, und `help-view.tsx` macht daraus
`/help/kindersuche`. Die Druckseite `/help/nfc/erste-schritte` bleibt davon unberührt.

Ein kompakter Einstieg „Frag ChatGPT“ steht oben in der festen Seitenleiste und im mobilen Kopf.
Er öffnet ChatGPT mit einem vorausgefüllten Prompt. Der Prompt nennt die Adresse der aktuell
geöffneten Hilfeseite. Auf einer Themenseite nennt er zusätzlich das aktuelle Thema. Dieses Thema
dient nur als Startpunkt; Fragen zur gesamten moto-Hilfe bleiben ausdrücklich möglich. Auf der
Übersicht startet der Prompt ohne Thema. Damit lässt sich der Übergang ohne eigenen GPT und ohne
API-Anbindung erproben. Der Parameter `prompt` ist in der offiziellen OpenAI-Dokumentation nicht
beschrieben und bleibt deshalb vorerst eine Prototyp-Annahme.

„Zurück zur App“ steht auf Desktop getrennt von den Hilfe-Funktionen unten in der festen
Seitenleiste. Mobil erscheint der kürzere Button „Zur App“ im Kopf. Der Rückweg führt zum internen
Pfad aus dem optionalen URL-Parameter `return_to`. Ohne diesen Parameter führt er zur
App-Startseite. Externe Ziele und andere Hilfe-Seiten sind als Rücksprungziel ausgeschlossen.

Eine Suche steht direkt unter „moto Hilfe“. Sie durchsucht Titel, Fragen, Kurzbeschreibungen,
Schritte und Tipps der typisierten Hilfe-Inhalte. Treffer erscheinen sofort als anklickbare
Themen in der vorhandenen Navigation. Die Suche läuft nur im Browser und braucht weder einen
eigenen Suchdienst noch einen zusätzlichen Index.

Alle Themenseiten verwenden im Prototyp den ausgewählten Dokumentationsstil. Die Darstellung nutzt
eine ruhige Typografie, gegliederte Abschnitte und eine Navigation „Auf dieser Seite“. Die feste
Themenseitenleiste und die moto-Farben bleiben erhalten. Ein zusätzlicher URL-Parameter ist dafür
nicht mehr nötig. Beispiel:

```text
/help/kindersuche?role=caregiver
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
Thema, Rolle sowie `nfc_enabled`, `presence_mode` und `group_mode` stehen in der URL. Rolle und
Konfigurationswerte werden im Hilfebereich nicht als Auswahl gezeigt: Die App setzt sie beim
Einstieg. Themenwechsel und alte Hash-Links erhalten den vollständigen Kontext; Hash-Links werden im
Prototyp clientseitig auf die passende Themenseite übertragen. `schoolyard` wird nicht übernommen.

**P4 · Responsive Prüfung** — Größe S
Die ausgewählte Seitenleisten-Lösung wird auf Desktop, Tablet und Smartphone geprüft. Navigation,
Direktlinks, Querverweise, der Rückweg zur App, der ChatGPT-Einstieg, die Suche und die
Dokumentationsdarstellung müssen anklickbar sein. Es gibt keine schreibenden Aktionen.

**P4a · ChatGPT-Einstieg erproben** — Größe S
Ein kompakter Link oben prüft Platzierung, Beschriftung und den Wechsel zu einem externen Chat. Der
Prompt übergibt die aktuelle Hilfe-Adresse und, falls vorhanden, das Thema. ChatGPT muss die
öffentliche Hilfeseite selbst aufrufen können. Der Link überträgt keine Daten aus dem angemeldeten
Bereich.

**P4b · Dokumentationsstil übernehmen** — Größe S, abgeschlossen
Der zunächst an „Ein Kind finden“ geprüfte Dokumentationsstil ist jetzt die Standarddarstellung für
alle Themenseiten des Prototyps. Alle Seiten nutzen weiterhin dieselben typisierten Prototyp-Daten.

**P4c · Themensuche erproben** — Größe S, abgeschlossen
Ein Suchfeld in der Seitenleiste und im mobilen Kopf filtert die vorhandenen Themen sofort. Es
durchsucht auch Kurzbeschreibungen, Schritte und Tipps, damit Aufgaben über ihre Handlungen
auffindbar sind. Eine leere Trefferliste bietet einen direkten Weg zurück zu allen Themen.

**P5 · Entscheidung dokumentieren** — extern abhängig
Das Team bestätigt die ausgewählte Seitenleiste oder benennt nötige Anpassungen. Ergebnis und
Begründung kommen in #2229. Erst danach wird die Lösung produktionsreif neu umgesetzt.

### Arbeitspakete der anschließenden Migration

Vier Stränge nach der Prototypentscheidung. **A** und **B** sind unabhängig voneinander und können
parallel laufen; **D5** ist ein Defekt und hängt an nichts.

### Strang A — Gerüst (kein Inhaltswechsel)

**A1 · `guide-data.ts` zerlegen** — gegenstandslos
Das Paket wollte die 3.000-Zeilen-Datei in eine Datei pro Kapitel zerlegen und dabei über einen
Barrel-Export die Namen `setupChapters`, `appChapters` und `nfcChapters` erhalten, damit kein
Konsument sich ändert. Beides ist überholt: die Datei ist gelöscht, die Kapitelsätze existieren
nicht mehr, und die Inhalte liegen als flache Artikelliste in `help-content.ts`. Damit entfällt
auch der Konflikt-Flaschenhals, um den dieses Paket herum geplant war — Artikel sind einzelne
Funktionen und stehen sich beim Merge seltener im Weg als Schritte in einem Kapitelbaum.
Die Nacharbeit an Regel und Skill, die A1 mittragen sollte, ist erledigt.

**A2 · Route pro Thema** — abgeschlossen
`/help/<topic>` ist flach (§7.1). Statt aus einem Kapitelbaum erzeugt zu werden, liegt hinter allen
Adressen **eine** Route: der optionale Catch-all `src/app/help/[[...topic]]/page.tsx`. Er bedient
`/help` (Einstieg), `/help/<thema>` und `/help/gruppe/<kategorie>`. Die Themen-Id ist der Slug und
kommt aus `HELP_TOPICS`. Die Route nimmt `role`, `nfc_enabled`, `presence_mode`, `group_mode` und
`return_to`; unbekannte Werte erzeugen keine Variante.
_Weiterleitungen:_ `/help/setup`, `/help/features` und `/help/nfc` liegen als permanente
Weiterleitungen in `next.config.js`. Alte Anker-Links brauchen keine Regel pro Thema: der Browser
nimmt das Fragment über die Weiterleitung mit, und `help-view.tsx` macht aus `/help#kindersuche`
die Adresse `/help/kindersuche`. Im Browser geprüft.
_Fertig:_ jede Id in `HELP_TOPICS` löst in jeder Einstellungs-Kombination auf
(`help-topics.test.ts`), die drei alten Adressen leiten weiter, ein unbekanntes Thema zeigt eine
Sackgassen-Seite mit Weg zurück.

**A3 · Seitenleiste und Querverweise** — abgeschlossen
Die Navigation steht als feste Seitenleiste und wird auf kleinen Bildschirmen zur ausklappbaren
Themenliste. Sie kommt nicht aus einem Kapitelbaum, sondern aus `group` je Artikel plus
`GROUP_ORDER_BY_ROLE` für die Reihenfolge der Kategorien. „Weitere Themen" am Seitenende speist
sich aus `related`. Bauteile aus `components/ui/` (Regel `frontend-ui-kit.md`). Themenlinks,
Querverweise und Suchtreffer tragen `role`, `nfc_enabled`, `presence_mode`, `group_mode` und
`return_to` weiter.
_Eine Falle, die bleibt:_ `related` läuft durch denselben Rollenfilter wie die Seitenleiste. Ein
Verweis auf einen Artikel, den die Rolle nicht sieht, verschwindet lautlos.

**A4 · Suchindex splitten** — gegenstandslos
Es gibt keinen Index. Die Suche filtert die geladenen Artikel der Rolle direkt im Browser über
Titel, Frage, Kurztext, Schritte und Tipps (`help-view.tsx`); fuse.js ist nicht mehr beteiligt.
Das 264-KB-Bundle ist mit `guide-data.ts` verschwunden. Dafür ist eine neue Stelle entstanden, die
Aufmerksamkeit braucht — siehe Abschnitt 8.

**A5 · PDF-Pipeline** — Größe S, abgeschlossen
Anders entschieden als hier zunächst geplant: die drei Guide-Seiten bleiben **nicht** als
Druckansicht stehen, sie sind entfernt. Eine Themenseite ist kein druckbares Handbuch, und ein PDF
pro Thema wollte niemand (§6). Übrig bleibt der gedruckte NFC-Onepager `/help/nfc/erste-schritte`:
er ist das Blatt im Versandkarton und damit ein eigenes Produkt, kein Hilfe-Artikel. Er liegt als
statische Route neben dem Catch-all und gewinnt gegen ihn, weil Next.js konkrete Segmente zuerst
prüft. `generate-guides.ts` rendert nur noch ihn; der CI-Schritt in `build.yml` bleibt unverändert.
_Fertig:_ `nfc-erste-schritte.pdf` entsteht weiter, `MIN_PDF_BYTES` bleibt der Wächter.

### Strang B — Kontextuelle Hilfe (unabhängig von A)

**B1 · `HelpTopicId` ableiten** — Größe S, Risiko niedrig
Literal-Union aus dem Kapitelbaum. Ein Tippfehler wird damit zum Compile-Fehler.
`HELP_TOPICS` ist die gemeinsame, typisierte Registry für Hilfe-Inhalte und App-Zuordnung; B3
prüft, dass jeder Eintrag in jeder Einstellungs-Kombination auflösbar ist.

**B2 · Kontextuelle Hilfe im Seitenkopf** — Größe S, Risiko niedrig
Der feste Seitenkopf der Mitarbeiter-App löst das Thema zentral aus dem aktuellen Pfad auf. Einzelne
Seiten können die Zuordnung über `helpTopic` überschreiben. Gerendert wird ein Icon-Link mit
`aria-label="Hilfe zu dieser Seite"`; bei Mauszeiger und Tastaturfokus erscheint derselbe Text als
Tooltip. Verlinkt **vor A2** auf `/help/features#<id>`, danach ohne Änderung an den Seiten auf
`/help/<id>`. Beim Einstieg aus der App setzt der zentrale Link
`nfc_enabled`, `presence_mode` und `group_mode` aus den bereits aufgelösten Mandanten-Metadaten sowie
`role` und den aktuellen internen Pfad als `return_to`.
Die Zwischenstufe ist vorbei: der Link zeigt auf `/help/<id>`, und diese Adresse ist jetzt der
Vertrag. Ein Umbenennen einer Themen-Id bricht ihn — B3(a) macht das zum Testfehler.
_Regeln:_ kein Thema → kein Symbol (nie ersatzweise auf die Startseite); immer gleicher Tab als
normaler `<Link>` — ungespeicherte Eingaben schützt der vorhandene `use-navigation-guard` von selbst
(§7.2); mobil nicht mit dem globalen Hilfe-Eintrag der Bottom-Nav konkurrieren.

**B3 · Zwei Wächter** — Größe S, Risiko niedrig
(a) Auflösbarkeit: jede verwendete `HelpTopicId` ergibt eine existierende URL.
(b) Abdeckung, **shrink-only**: jede `page.tsx` unter `(protected)` ist zentral einem Thema
zugeordnet oder steht auf einer Ausnahmeliste, die nur kürzer werden darf — Muster wie
`serialTestBaseline`.
(c) Kontextvertrag: nur die dokumentierten Werte der drei Konfigurationsparameter erzeugen Varianten;
alle internen Hilfe-Links erhalten gültige Parameter und `return_to` unverändert.

**B4 · Seiten verdrahten** — Größe M, laufend
Die zentrale Pfadzuordnung enthält zunächst alle Seiten der neun Prototyp-Themen. Weitere Seiten
folgen schrittweise mit dem Ausbau der Inhalte, beginnend beim größten Support-Aufkommen. Seiten ohne
passendes Thema zeigen ausdrücklich kein Hilfe-Symbol. Die Ausnahmeliste aus B3 schrumpft dabei.

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
_Fertig wenn:_ eine Betreuungskraft findet DATEV, Datenverwaltung und Anmeldephasen nicht zwischen
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

| Risiko                                             | Warum                                                                                          | Umgang                                                                                                                                    |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| **A1 kollidiert mit jedem offenen Feature-Branch** | 88 % der Guide-Änderungen laufen im Feature-PR mit; 36 Konflikte gab es schon ohne Umbau       | In **einem** PR, angekündigt, zügig gemergt. Nicht über mehrere Tage offen halten. Ideal: an einem Tag mit wenigen offenen Guide-Branches |
| Alte Anker-URLs brechen                            | PDFs, Support-Mails, Suchtreffer im Umlauf                                                     | Weiterleitungen in A2 sind Pflicht, nicht Kür                                                                                             |
| ~~PDF-Erzeugung bricht~~                           | entfällt: es wird nur noch der gedruckte NFC-Onepager gerendert (A5)                           | `MIN_PDF_BYTES` bleibt der Wächter                                                                                                        |
| ~~Regelwerk veraltet~~                             | erledigt: Regel und Skill beschreiben `help-content.ts` und die Themenhilfe                    | beim nächsten Strukturwechsel wieder mitziehen                                                                                            |
| Themen-Ids werden zum Vertrag                      | B2 bindet App-Seiten an Themen; ein Umbenennen bricht den Link                                 | B3(a) macht es zum Compile-/Testfehler statt zum stillen Bruch                                                                            |
| Screenshot-Drift                                   | 77 Bilder, neuer Schnitt macht einen Teil ungültig                                             | In D3 mitplanen, nicht nachgelagert                                                                                                       |

## 6. Was wir NICHT bauen

- Keine mandantenspezifische Hilfe und kein PDF pro Schule (ADR 0009).
- Keine Filterung nach allen Mandanten-Einstellungen. Nur `nfc_enabled`, `presence_mode` und
  `group_mode` werden als URL-Kontext übergeben; auch sie blenden Themen nicht hart aus.
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

**7.5 Situations-Einstiege („Ich möchte …") kommen nach D3**, nicht in den ersten Wurf. Sie wären ein
dritter Weg in die Hilfe neben den Rollenkarten (D2) und den Hilfe-Symbolen in der App (B2). Gute
Situations-Einstiege lassen sich erst schreiben, wenn die Themen neu geschnitten sind — vorher legt
man eine aufgabenförmige Schicht über softwareförmige Inhalte und wiederholt damit genau die Kritik
aus #2229 eine Ebene höher.

## 8. Was die Umstellung offen lässt

Die Hilfe liegt unter `/help`, das alte Guide-System ist entfernt. Offen bleibt:

- **Lehrkräfte.** Die 15 Artikel der Rolle `teacher` stehen, sind aber noch nicht am laufenden
  moto schule nachgeprüft: die genannten Beschriftungen stammen aus dem Code, nicht vom Bildschirm.
  Dafür muss eine Leitung einen Lehrkraft-Zugang anlegen. Bis dahin gilt für diese 15 Artikel
  nicht, was `.claude/rules/help-guide-sync.md` verlangt („die Beschriftungen am echten Bildschirm
  ablesen").
- **Screenshots.** Die 87 Dateien unter `public/help/screens/` gehören zum alten Guide; kein
  Artikel setzt `image`. Sie bleiben liegen, weil sie die in `frontend-ui-kit.md` genannte
  visuelle Referenz sind — aber niemand zieht sie mehr nach. Entweder kommen sie in die Artikel
  zurück (D3) oder sie brauchen einen eigenen Ort.
- **Eröffnungssalden.** `/database/personal/opening-balances` springt ohne das Recht
  `time_tracking:manage` still auf `/database/personal` zurück, statt die Verbotsseite zu zeigen.
  Solange das so ist, lässt sich der Ablauf nicht abschreiten und der Artikel nicht schreiben.
- **Leere Geräte-Einstellungen.** Ohne NFC zeigt der Einstellungs-Reiter `Geräte` nichts an,
  erscheint aber. Eine Anleitung kann das nur beschreiben, nicht heilen.
- **`help-content.ts` liegt im Bündel jeder App-Seite.** `ContextHelpLink` ruft `getHelpTopics()`
  nur, um eine einzige Frage zu beantworten: Gibt es zu diesem Thema einen Artikel für diese Rolle?
  Dafür zieht es die vollständige Inhaltsdatei samt allen Artikeltexten und rund 60 Symbolen aus
  lucide-react in das Client-Bündel jeder Seite mit Seitenkopf. Das ist derselbe Befund wie in
  Abschnitt 1 zum alten Suchindex, nur an einer breiteren Stelle: vorher traf es die Hilfeseiten,
  jetzt die App.
  _Sichtbar geworden an:_ `time-tracking/page.test.tsx` scheiterte an `KeyRound`, weil der
  Symbol-Mock der Seite die Symbole der Hilfe-Inhalte nicht kennen kann. Der Test vertritt jetzt
  den Hilfe-Link, wie `header.test.tsx` es schon tat — das behebt den Test, nicht die Ursache.
  _Vorschlag:_ die Sichtbarkeit in ein eigenes, inhaltsfreies Modul ziehen (Thema → Rollen und die
  drei Einstellungsmengen), das `ContextHelpLink` allein importiert. Die Artikeltexte blieben dann
  im Bündel der Hilfe, wo sie hingehören. Messen vor dem Umbau: `docs/agents/frontend-performance.md`
  nennt die Budgets.
