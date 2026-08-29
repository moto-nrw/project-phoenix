# Der Hilfebereich bleibt typisierte Daten in der moto-App

Status: accepted. Gilt für den schul-öffentlichen Hilfebereich unter `/help`
(Ersteinrichtung, Die App im Alltag, NFC Erste Schritte, NFC & Tablets) und für
jede künftige Erweiterung daran. Entstanden aus der Recherche zu #2229.

## Kontext

#2229 verlangt einen Hilfebereich, der schnelle Antworten auf konkrete Fragen
aus dem OGS-Alltag gibt: kurze Themenseiten statt langer Dokumente, dauerhafte
Navigation, Einstiege nach Rolle und Aufgabe. Das Issue nennt Mintlify als
Kandidaten und fragt, ob eine fertige Dokumentationslösung den Eigenbau ersetzen
soll.

Das beschriebene Problem ist real. `/help/features` rendert sieben Kapitel mit
rund 60 Themen auf einer einzigen Route; das daraus erzeugte PDF hat 91 Seiten.
Die Kapitelüberschriften sind eins zu eins die Sidebar-Bereiche der App
(„Home", „Alle Kinder", „Aktivitäten", „Kategorien", „Räume"), also genau die
Software-Perspektive, die das Issue kritisiert.

Messung am 29.08.2026 an `frontend/src/components/help/`:

- `guide-data.ts` hat 3.001 Zeilen (264 KB), 23 Kapitel und 108 Schritte,
  typisiert als `GuideChapter` → `GuideStep`.
- Die Datei hat **652 Commits von sieben Autoren, davon null Nicht-Entwickler.**
- Von den letzten 300 Nicht-Merge-Commits kamen **264 (88 %) zusammen mit
  Produktcode**, nur 36 waren reine Doku-Commits. Ursache ist
  `.claude/rules/help-guide-sync.md`: der Guide wird im selben PR wie das Feature
  geändert.
- In dieser einen Datei wurden **36 Merge-Konflikte** aufgelöst.
- Prosa macht rund 1.600 der 3.001 Zeilen aus. Das einzige vorhandene Markup
  sind Backticks für `<code>`, 1.697 Vorkommen. Kein Fett, keine Links, keine
  verschachtelten Listen.
- Die Struktur wird schmal genutzt: `steps` 131, `icon` 95, `callout` 84,
  `image` 80 — dagegen `printCompact` 16, `gallery` 14, `searchTerms` 4,
  `checklist` 3. Wiederverwendung über geteilte Konstanten gibt es genau
  einmal (Android-Installation, dreifach genutzt).
- Der Suchindex (fuse.js) wird auf Modulebene in `guide-search.ts` gebaut und
  von einer `"use client"`-Komponente importiert. Dadurch liegt die komplette
  264-KB-Datei mit allen Texten aller Guides im Client-Bundle jeder Hilfeseite.
- Abgeleitet aus denselben Daten: Suche, die Deep Links, das per Playwright
  erzeugte PDF (`build.yml`) und der Drift-Wächter `help-search-sync.test.ts`.

Zu Mintlify, dem im Issue genannten Kandidaten:

- Es gibt eine **kostenlose Stufe** (Starter: 0 $, volle Publishing-Plattform
  mit eigener Domain, Web-Editor, Auth, API-Playground), **begrenzt auf fünf
  Editor-Seats**. Pro kostet 450 $/Monat, EU-Hosting und Self-Hosting gibt es
  nur im Enterprise-Tarif. An `guide-data.ts` haben bisher **sieben Personen**
  committed — die kostenlose Stufe passt nicht auf die heutige Teamgröße.
- Mintlify baut **aus einem GitHub-Repo** und unterstützt Doku in einem
  Unterverzeichnis eines Monorepos. Die Inhalte müssten das Repo also *nicht*
  verlassen, und die Same-PR-Kopplung wäre technisch erhaltbar. Das Format ist
  dabei MDX/Markdown.
- Gerendert und ausgeliefert wird jedoch **außerhalb dieser Anwendung**: eigenes
  Theme, eigene Domain, eigener Deploy-Auslöser.

## Entscheidung

- **Kein Dokumentationsanbieter.** Der Hilfebereich bleibt Teil dieser
  Anwendung. Ausschlaggebend ist **nicht der Preis** — es gibt eine kostenlose
  Stufe — und auch nicht, dass Inhalte das Repo verlassen müssten; das müssen
  sie bei Mintlify nicht. Ausschlaggebend ist dreierlei:
  **(a)** Mintlify rendert MDX. Die Anbieterfrage ist damit der Formatfrage
  nachgelagert und mit der MDX-Absage weiter unten beantwortet.
  **(b)** App und Hilfe würden getrennt ausgeliefert. Heute liegt die Hilfe im
  selben Image, auf denselben Commit-SHA gepinnt, über denselben SOPS-Deploy;
  die Same-PR-Pflicht aus `help-guide-sync.md` wirkt deshalb bis in die
  Auslieferung. Bei getrennten Deploys kann die Hilfe ein Feature beschreiben,
  das noch nicht live ist, und umgekehrt.
  **(c)** Die Hilfe wäre eine fremde Website mit fremdem Theme — gegen
  `frontend-ui-kit.md`, und die kontextuellen Deep-Links aus ADR 0009 würden zu
  Sprüngen auf eine andere Domain.
  Nicht ausschlaggebend, aber zu vermerken: EU-Hosting nur im Enterprise-Tarif
  ist hier ein AVV-Thema (Verbindungsdaten lesender Eltern und Beschäftigter),
  kein Datenresidenz-Thema — die Hilfeinhalte selbst sind öffentlich und
  personenfrei.
- **Kein zweiter Doku-Build.** Auch ein eigenständiger Generator (Starlight,
  Docusaurus, Nextra) ist abgelehnt: zweiter Build, zweiter Deploy, zweites
  Designsystem, PDF- und Screenshot-Pipeline neu — gegen Navigations-Chrome, das
  wir ohnehin im moto-Erscheinungsbild bauen müssten.
- **Kein MDX. Die Inhalte bleiben typisierte TypeScript-Daten.** Das
  Hauptargument für MDX ist bessere Schreibbarkeit für Nicht-Entwickler — dafür
  gibt es in 652 Commits keinen einzigen Beleg. Zudem bleiben die 88 %
  feature-gekoppelten Einträge strukturell bei den Entwicklern: einen Eintrag
  für ein Feature, das in keinem gemergten Branch existiert, kann eine externe
  Redaktion nicht schreiben. Der Compiler garantiert im Gegenzug, dass jeder
  Schritt eine Bildunterschrift hat, `tone` gültig ist und `icon` auf eine
  existierende Komponente zeigt.
- **Kein Doku-Framework als UI-Schicht**, auch nicht Fumadocs. Dessen Wert
  steckt überwiegend in `fumadocs-mdx` und im mitgelieferten Theme. Das erste
  entfällt mit der MDX-Absage, das zweite kollidiert mit
  `.claude/rules/frontend-ui-kit.md`.
- **Positiv gesagt: der Hilfebereich ist eine Handvoll normaler Seiten in
  dieser Next.js-App.** Es gibt kein Doku-Framework — die Antwort auf „welches
  nutzen wir" lautet „keins", und das ist die Entscheidung, kein Versäumnis.
  Gebaut wird mit dem, was die App ohnehin hat: Next.js App Router und React
  für Seiten und Routing, eigene Komponenten aus dem moto-UI-Kit mit Tailwind
  für die Darstellung, der typisierte `GuideChapter` → `GuideStep`-Baum als
  Inhalt, fuse.js für die Suche über einen daraus abgeleiteten Index,
  lucide-react für Icons, `next/image` für die Screenshots, Playwright plus
  pdf-lib für die PDFs, vitest für die Wächter. Seitenleiste,
  Inhaltsverzeichnis und Suche sind Eigenbau.
- **Keine neue Abhängigkeit für diesen Bereich**, solange dieser ADR gilt. Wird
  eine gebraucht, ist das eine Abweichung und gehört in der PR-Beschreibung
  begründet.

## Folgen

- `guide-data.ts` wird in eine Datei pro Thema zerlegt. Die 36 Merge-Konflikte
  sind eine Folge der einen großen Datei, nicht des Formats; der Split ist
  zugleich Voraussetzung für Route-pro-Thema.
- Die 108 Schritte bekommen eigene Routen. Damit fällt der Suchindex pro Guide
  auseinander und das 264-KB-Client-Bundle verschwindet als Nebenwirkung.
- Die PDF-Erzeugung muss beim Routen-Split nachziehen: `generate-guides.ts`
  rendert heute drei Guide-Seiten. Bei Route-pro-Thema muss sie entweder weiter
  pro Guide bündeln oder pro Thema rendern. Das ist die Stelle, die am ehesten
  überrascht.
- Die Entscheidung bleibt billig umkehrbar. Aus zerlegten typisierten Dateien
  ist ein späterer MDX-Wechsel deutlich günstiger als aus dem Monolithen.
- **Bedingung für eine Revision:** sobald eine benannte Person ohne
  Entwicklerrolle dauerhaft Hilfeinhalte schreiben soll, ist die MDX-Frage neu
  zu stellen — dann aber zusammen mit Vorschau-Umgebung und Review-Weg, nicht
  als reine Formatwahl.
- Die inhaltliche Arbeit aus #2229 (neue Informationsarchitektur, Einstiege nach
  Rolle und Aufgabe, OGS-nahe Sprache nach `moto-einfache-sprache`) ist von
  dieser Entscheidung unberührt und bleibt das eigentliche Volumen.
