# Bauarten des OGS-Portals — verbindlicher Spec für das Innere einer Seite

`TENANT-PAGE-SPEC.md` regelt den Rahmen: Kopfkarte, Statuszeile, Suche,
Reiter, Rhythmus. Ab dem Inhalt hörte die Regel bisher auf, und genau dort
laufen die Seiten auseinander. Dieses Dokument schließt die Lücke.

Grundsatz: **Es gibt vier Bauarten. Jede Fläche entscheidet sich für genau
eine und erfindet innen nichts Eigenes.** Wer eine fünfte braucht, ändert
diesen Spec — nicht seine Seite.

Nicht betroffen: Eltern-Portal, Schul-Portal, Operator-Portal.

## Warum

Aus der Bestandsaufnahme vom 28.08.2026 (drei Audits über alle Tenant-Seiten):

- Ein Klick auf eine optisch identische Kachel führt je nach Seite auf eine
  eigene Route, in ein Slide-over, in ein zentriertes Modal oder nirgendwohin.
- Kind und Mitarbeiter haben je zwei getrennte Detailansichten ohne
  Querverweis, mit unterschiedlicher Anatomie und unterschiedlichem
  Editier-Code. Der Raum hat drei Zustände, einer davon eine tote Route.
- „Neu anlegen" fehlt auf den Übersichtsseiten von Kindern, Personen und
  Räumen, existiert aber bei Aktivitäten.
- Tabellenzeilen sind auf manchen Seiten klickbar, auf anderen nicht.
- Sechs Flächen bauen ein eigenes Ladeskelett neben dem eingebauten;
  derselbe Ladefehler wird einmal über `error`, einmal über `empty` gemeldet.
- Fünf verschiedene Lösch-Bestätigungen, drei Fehlerdarstellungen.
- Drei Planungsflächen teilen dasselbe Rasterbauteil und verhalten sich
  darin unterschiedlich (Klick auf leer, Legende, Export).

Nichts davon ist ein Fehler im Einzelfall. Zusammen sind sie der Grund,
warum das Portal auf jeder Seite neu gelernt werden muss.

---

## Bauart 1 — Sammlung

Eine Liste von Objekten eines Typs. Beispiele: Kinder, Mitarbeitende, Räume,
Aktivitäten, Gruppen, Rollen, Geräte, Dateien, Nachrichten, Anfragen.

1. **Eine Darstellung pro Objekttyp, an allen Breakpoints dieselbe.** Kacheln
   für Objekte, die man am Namen und einem Status erkennt (Kind, Person,
   Raum). Tabelle für Objekte, die man an mehreren Spalten vergleicht
   (Anmeldephasen, Geräte, Displays). Kein Wechsel Tabelle/Kacheln je
   Breakpoint — eine Tabelle wird mobil scrollbar, nicht zu Karten.
2. **Kachel und Zeile öffnen immer die Objektansicht (Bauart 2), und immer
   auf demselben Weg.** Der Weg ist portalweit einer: die Objektroute. Kein
   Slide-over als Ersatz für eine Detailseite, kein zentriertes Modal als
   Detailansicht. Eine nicht klickbare Zeile in einer Liste klickbarer Zeilen
   gibt es nicht.
3. **„Neu anlegen" steht immer als Kopf-Aktion**, auf jeder Sammlung ihres
   Typs, mit demselben Wort („Kind anlegen", „Raum anlegen"). Es gibt keine
   Sammlung, auf der man das Objekt sehen, aber nicht anlegen kann, während
   es an anderer Stelle anlegbar ist.
4. **Zeilenaktionen ausschließlich im Kebab der Zeile.** Keine Icon-Reihe,
   keine Aktion, die nur beim Überfahren erscheint. Was das Objekt betrifft
   und nicht die Liste, gehört in die Objektansicht.
5. **Mehrfachauswahl ist eine Eigenschaft der Bauart, nicht der Seite.** Wo
   sie fachlich sinnvoll ist, wird sie überall gleich ausgelöst
   (Kopf-Aktion „Auswählen" schaltet den Auswahlmodus). Sie fehlt nicht auf
   sechs von sieben baugleichen Registern.
6. **Eine Paginierung im Portal.** Serverseitig geblättert wird über dasselbe
   Bauteil an derselben Stelle. Kein Nebeneinander von „Zurück/Weiter",
   „Mehr laden" und unbegrenztem Rendern.
7. **Leer, Laden und Fehler kommen aus `TenantPage`** (`empty`, `loading`,
   `error`). Kein handgebautes `EmptyState` mitten im Inhalt, kein eigenes
   Skelett, keine domänenspezifische Leerzustands-Komponente.
8. **Der Leerzustand ist der nächste Schritt.** Titel, ein Satz, und die
   Aktion, die den Zustand beendet — nie nur eine Feststellung.

## Bauart 2 — Objekt

Ein einzelnes Ding. **Pro Objekttyp gibt es genau eine Objektansicht im
gesamten Portal**, unter einer Route, unabhängig davon, aus welcher Sammlung
man kommt. Verwaltungsfelder sind ein Reiter darin, sichtbar nach Recht —
keine zweite Ansicht.

1. **Anatomie, fest:** Identitätskopf (Name, Avatar, aktueller Status) →
   Reiter → Feldgruppen → Aktionen im Kebab des Kopfes.
2. **Felder kommen aus `ui/detail-modal-components`** (`DataField`,
   `DataGrid`, `InfoSection`). Kein lokales `<dt>/<dd>`-Gitter, keine
   handgebaute Feldzeile.
3. **Bearbeitet wird am Objekt, nicht daneben.** Ein Reiter wechselt in den
   Bearbeiten-Zustand und zurück. Kein Modal pro Feldgruppe, kein Modal für
   das ganze Objekt, kein Modal über einem Modal.
4. **Ein Speichern-Knopf pro Bearbeiten-Zustand**, unten, mit dem Wort
   „Speichern". Automatisches Speichern gibt es außerhalb der Einstellungen
   nicht.
5. **Fehler stehen im `Alert` oben im Bearbeiten-Bereich und, wo zuordenbar,
   am Feld.** Kein Toast als einzige Fehlermeldung, kein roter Absatz.
6. **Löschen ist portalweit ein Muster:** `ConfirmDeleteModal`. Die
   Texteingabe-Bestätigung ist die Stufe für Unwiderrufliches mit
   Datenverlust, sonst reicht die einfache Rückfrage. Kein `window.confirm`,
   kein Umschalten des Formular-Footers, kein eigenes Löschmodal je Domäne.
7. **Keine deaktivierten Platzhalter-Aktionen.** Was es nicht gibt, steht
   nicht im Menü.
8. **Zurück** immer über den Kopf der `TenantPage` (`back`/`backHref`).

## Bauart 3 — Werkzeug

Flächen, auf denen über Zeit gearbeitet wird: Betreuungsplan, Dienstplan,
Vertretung, Kalender, Zeiterfassung, Statistik, Tagesauswertung, Abrechnung.

1. **Zeitnavigation immer im `searchSlot` der Kopfkarte, immer als
   `PlanningContextBar`**, mit `SegmentedControl` für die Auflösung und
   demselben Wochenlabel-Format überall. Keine Pfeilknöpfe im Inhalt, kein
   zweites Label-Format.
2. **Gleiches Rasterbauteil heißt gleiches Interaktionsversprechen.** Wo ein
   Klick auf eine leere Fläche anlegt, tut er das auf jeder Fläche mit
   diesem Raster. Wo nicht, nirgends.
3. **Jede Fläche mit Farbcodierung trägt eine `PlanLegend`.** Farbige Blöcke
   ohne Legende gibt es nicht.
4. **Export und Drucken stehen im Kebab der Kopfkarte**, unter
   „Drucken oder exportieren", auf jeder Werkzeugfläche, die Daten zeigt, die
   man mitnehmen können muss. Keine eigenen Knopfreihen, keine Fläche, der
   der Export fehlt, während die Nachbarfläche ihn hat.
5. **Laden, Fehler und Leer über `TenantPage`.** Ein eigenes Skelett braucht
   eine Begründung im PR und darf nicht die Fehler- und Leerzustände
   mitnehmen. Ein Ladefehler ist `error`, nie `empty`.
6. **Zwei Werkzeuge werden nicht per Reiter in einer Seite gemischt**, wenn
   sie unterschiedliche Zeitnavigation, Legende oder Anlege-Logik haben.
   Entweder sie werden angeglichen, oder es sind zwei Seiten.

## Bauart 4 — Einstellungen

Konfiguration einer Schule.

1. **Die einzige Bauart, die automatisch speichert.** Das Verhalten wird auf
   der Fläche benannt („Änderungen werden sofort gespeichert").
2. Reiter über `TenantPage`, Felder aus dem Settings-Schema, keine
   handgebauten Karten außer den dokumentierten Ausnahmen.
3. Was eine Voraussetzung hat, nennt sie dort, wo man sie einschaltet.

---

## Querregeln

**Farbe.** Die Navigation ist einfarbig. Farbe bedeutet ausschließlich
Status: grün anwesend, grau nicht da, rot krank oder Fehler, lila genehmigt
abwesend, orange braucht Aufmerksamkeit. Werte kommen aus `LOCATION_COLORS`
oder `moto-*`-Klassen — kein roher Hexwert, auch nicht als Fallback, auch
nicht wenn er zufällig mit dem Token übereinstimmt.

**Wörter.** Ein Begriff, ein Wort, in Seitenleiste, Brotkrume, Seitentitel,
Knopf, Hilfe und E-Mail. Zwei sichtbare Namen im Portal teilen keinen
Wortstamm ohne sichtbare Abgrenzung (`.claude/rules/verstaendlichkeit.md`).

**Zustände.** `loading`, `error`, `empty` sind Eigenschaften der `TenantPage`
und werden dort belegt. Fehlende Rechte sind ein Zustand, kein Fehler
(`ForbiddenPage`).

---

## Ratschen

Ohne Prüfung driftet das zurück. Jede Regel oben, die maschinell prüfbar ist,
bekommt eine shrink-only Baseline analog zum bestehenden
`oxlint-plugin-ui-kit.mjs`:

1. `bauart/one-detail-per-type` — kein zweiter Detailbaum für einen
   Objekttyp.
2. `bauart/no-local-field-grid` — kein lokales `<dt>/<dd>`-Feldgitter
   außerhalb `ui/detail-modal-components`.
3. `bauart/one-delete-confirm` — nur `ConfirmDeleteModal`; kein
   `window.confirm`, kein `ConfirmationModal` für Löschen.
4. `bauart/no-own-skeleton` — kein eigenes Seiten-Skelett neben den
   `TenantPage`-Zuständen.
5. `bauart/no-raw-status-hex` — keine rohen Hexwerte für Status- und
   Planungsfarben.
6. `bauart/no-disabled-menu-item` — keine dauerhaft deaktivierten
   Menüeinträge.
7. Erweiterung von `tenant-page-scaffold.test.ts`: jede Seite deklariert ihre
   Bauart, und die Zuordnung ist vollständig.

## Reihenfolge der Umsetzung

1. **Rahmen fertig** — die verbleibenden Seiten ohne `TenantPage` nachziehen,
   die sechs eigenen Ladeskelette auf die Gerüst-Zustände umstellen,
   `error`/`empty` korrekt belegen.
2. **Bauart 2 zuerst** — eine Objektansicht je Typ, die Doppelbäume von Kind,
   Mitarbeiter und Raum zusammenführen, Verwaltungsfelder als Reiter. Das ist
   zugleich die halbe Navigationsentscheidung.
3. **Navigation und Wörter** — Datenverwaltung auflösen, Seitenleiste auf
   etwa acht Einträge, Wörterbuch mit Ratsche, Weiterleitungen für alte
   Pfade, Hilfe-Anleitung im selben Zug.
4. **Bauart 1 und 3** — Sammlungen und Werkzeuge angleichen.
5. **Farbe und Ruhe** — einfarbige Navigation, Statusfarben aufräumen.
6. **Selbsterklärung** — jeder Leerzustand als nächster Schritt, jede
   Voraussetzung am Schalter.

Die Ratsche einer Stufe wird mit der Stufe eingeführt, nicht danach.

---

# Teil 2 — Navigation und Wörter (Ziel-Zustand)

Nachtrag vom 28.08.2026: Die Navigation ist Teil des Umbaus. Ziel ist, dass
eine Sache genau einen Ort hat und die Seitenleiste in einem Blick lesbar ist.

## Der Grundsatz

**Ein Objekt, ein Ort.** „Datenverwaltung" wird aufgelöst. Sie war kein Ort,
den eine OGS-Leitung im Kopf hat, sondern eine Beschreibung der Datenbank —
und sie hat jede Entität verdoppelt: das Kind wohnte unter „Alle Kinder" UND
unter „Datenverwaltung → Kinder", mit zwei Ansichten, zwei Bearbeiten-Wegen
und ohne Querverweis.

Verwaltung ist ab jetzt ein **Modus am Objekt**, kein zweiter Baum: die
Stammdaten eines Kindes sind ein Reiter im Kind, sichtbar nach Recht. Wer das
Recht nicht hat, sieht den Reiter nicht — nicht eine zweite Seite, die er
nicht öffnen kann.

## Die Seitenleiste

Neun Bereiche plus drei Einträge am Fuß. Reihenfolge fest:

1. **Start**
2. **Kinder** — Sammlung, Objekt, Reiter „Abwesenheiten"; Stammdaten als
   Reiter im Kind
3. **Mitarbeitende** — Sammlung, Objekt; Stammdaten, Dokumente, Abwesenheiten
   als Reiter in der Person
4. **Räume** — Sammlung, Objekt, Reiter „Aufsicht heute"
5. **Aktivitäten** — Sammlung, Objekt
6. **Planung** — Betreuungsplan · Dienstplan · Vertretung · Tageslisten ·
   Zeiträume · Mein Kalender
7. **Eltern** — Nachrichten · Mitteilungen · Konto-Anfragen · Essensplan ·
   Anmeldungen
8. **Team** — Team-Chat · Dateien
9. **Auswertung** — Statistik · Zeiterfassung · Abrechnung · Tagesauswertung

Am Fuß: **Notfall** · **Hilfe** · **Einstellungen**. In den Einstellungen
wohnen die Register, die reine Konfiguration sind und kein Tagesgeschäft:
Gruppen, Rollen, Berechtigungen, Geräte, Info-Displays, Exporte,
Jahrgangswechsel.

Was damit verschwindet: der Bereich „Datenverwaltung", der Doppeleintrag
„Anfragen" neben „Konto-Anfragen", der Einzeleintrag „Gruppenzugriff" (er
wird ein Reiter bei den Mitarbeitenden), und die dynamisch eingehängten
Gruppen- und Raum-Kinder in der Seitenleiste — eine Navigation, die mit den
Daten wächst, ist keine Navigation mehr.

## Die Wörter

Ein Begriff, ein Wort, überall gleich: Seitenleiste, Brotkrume, Seitentitel,
Knopf, Hilfe, E-Mail. Verbindlich:

| Statt | Ab jetzt |
|---|---|
| „Home" / „Startseite" / „Suchen" | **Start** |
| „Alle Kinder" / „Kindersuche" / „Kinder" | **Kinder** |
| „Mitarbeiter" / „Personal" / „Lehrkräfte" | **Mitarbeitende** |
| „Gruppenzugriff" | **Vertretungszugriff** (Reiter bei Mitarbeitende) |
| „Betreuungsangebote" | **Angebote** (im Bereich Anmeldungen) |
| „Tagesauswertung" | **Tagesbericht** |
| „Mein Kalender" | **Kalender** |

Zwei sichtbare Namen teilen keinen Wortstamm ohne sichtbare Abgrenzung.
Der Wortstamm „Betreuungs-" darf nur noch EINMAL als Navigationsname
vorkommen (Betreuungsplan).

## Farbe in der Navigation

Die Seitenleiste ist einfarbig. Heute tragen die Einträge elf verschiedene
Akzentfarben, die nichts bedeuten — und entwerten damit das Rot, das „krank"
heißt. Der aktive Eintrag wird durch Fläche und Schriftschnitt markiert, nicht
durch eine eigene Farbe je Bereich. Farbe bleibt ausschließlich Status.

## Nichts darf ins Leere laufen

Jede alte Route bleibt als Weiterleitung erhalten. Schulen haben Lesezeichen,
und die Hilfe-Anleitung nennt Pfade. Eine Weiterleitung wird nicht später
aufgeräumt, sie gehört zum Umbau.

Die Hilfe-Anleitung (`components/help/guide-data.ts`) und ihre Screenshots
werden im selben Zug nachgezogen — jeder Schritt, der einen Pfad oder einen
Namen nennt, den dieser Umbau ändert.

---

# Teil 3 — Ruhe: zwei Ebenen, eine Kopffläche

Nachtrag vom 28.08.2026 nach der Sichtprüfung im Browser. Die Seiten waren
regelkonform und wirkten trotzdem zusammengeschustert. Ursache waren nicht die
Bauteile, sondern wie sie auf dem Grund lagen.

## 1. Es gibt zwei Ebenen: Grund und Fläche

Der gemusterte Hintergrund ist Grund. **Auf dem Grund steht kein Text.** Keine
Überschrift, kein Erklärsatz, keine Zahl, keine Reiterzeile, kein
Bedienelement. Alles davon sitzt auf einer Fläche aus dem Kit.

Der häufigste Verstoß ist `SectionCard bare`: die Karte verzichtet auf ihre
Fläche, und ihr Titel samt Zeitraumwahl schwebt danach auf dem Punktraster.
`bare` ist nur zulässig, wenn der Abschnitt AUSSCHLIESSLICH andere Karten
enthält und selbst weder Titel noch Text trägt.

## 2. Die Kopfkarte ist eine geschlossene Fläche

Titel, Statuszeile, Reiter und Suche gehören in EINE Karte, in genau dieser
Reihenfolge (zur Stelle der Reiter siehe unten). Die Karte ist rundum gerahmt
und gerundet — eine offene Kante hängt in der Luft, solange der Inhalt erst
24 px darunter beginnt.

Verboten ist die frühere Bauart: eine Reiterzeile zwischen Kopf und Inhalt,
frei auf dem Grund, mit einer Haarlinie, die im Nichts endet.

**Die Reiter stehen ÜBER der Suchzeile**, direkt unter Titel und Statuszeile.
Der Reiter bestimmt, WAS man ansieht; die Suche filtert DARIN. Unter der Suche
sitzt er in derselben Zone wie Suchfeld und Filter und wird als ein weiteres
Filterelement gelesen — daran ändert weder Farbe noch Größe etwas.

**Die Grundlinie gehört dem Band, nicht dem einzelnen Reiter.** Eine Haarlinie
läuft über die volle Kartenbreite; der aktive Reiter färbt nur sein Stück davon
grün ein (`border-b-[3px]`, `text-base`, `pb-3`, Abstand untereinander `gap-6`).
Zwei Gründe:

- *Abstände.* Trägt nur der aktive Reiter einen Strich, richtet sich die Zeile
  an etwas aus, das den übrigen fehlt: unter deren Text steht fast doppelt so
  viel Luft. Mit der Linie am Band sind alle Reiter gleich hohe Kästen.
- *Verständlichkeit.* Die Linie verbindet den Reiter sichtbar mit dem Inhalt
  darunter. Eine getönte Pille oder eine geschlossene Segment-Spur sagt das
  nicht — beide lesen sich als „wähle einen Wert", nicht als „wechsle die
  Ansicht". Genau daran scheiterten die ersten beiden Fassungen.

Kein `-mb-px` an den Reitern. Der gebündelte Reiter (`menu`) rendert eine
zusätzliche Hülle um seine Schaltfläche; ein negativer Rand greift dort an der
inneren Schaltfläche und verschiebt genau diesen einen Reiter um ein Pixel.

## 3. Höchstens vier Seitenreiter

Mehr Reiter sind eine Werkzeugleiste, keine Orientierung. Was täglich
gebraucht wird, steht als Reiter; alles Seltene bündelt sich hinter EINEM
Reiter mit Menü (`TenantPageTab.menu`), benannt nach dem, was es enthält —
in der Regel „Verwaltung". Der gebündelte Reiter zeigt den Namen des offenen
Untereintrags, damit man sieht, wo man ist.

## 4. Farbe bleibt Status

Unverändert gültig (Querregel Farbe). Ergänzend aus der Sichtprüfung: eine
Kennzahl, die nur groß ist, ist kein Warnzustand. Orange und Rot an einer Zahl
bedeuten, dass jemand handeln muss — sonst bleibt sie neutral.
