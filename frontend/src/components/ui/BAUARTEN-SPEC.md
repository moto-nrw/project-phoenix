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

   **Wann ein Pane neben der Liste zulässig ist (#3115):** nur, wenn es die
   einzige Objektansicht des Typs im Portal ist (Gruppe, Gerät, Rolle,
   Aktivität, Berechtigung, die Kataloge der Datenverwaltung). Dann steht
   die Auswahl in der Adresse (`?group=`), damit ein Eintrag verlinkbar
   bleibt; `useUpdateUrlParams` schreibt sie per `router.replace`, ein
   Zurück im Browser springt also zur vorigen Seite, nicht zum vorigen
   Eintrag. Sobald der Typ eine Route hat (Kind, Person, Raum,
   Elternmitteilung), verlinkt die Zeile dorthin (`DatabaseListItem href`,
   `?from=` trägt den Rückweg zur Sammlung samt Filtern) und es gibt kein
   Pane, kein Slide-over und keinen zweiten Detailbaum daneben. Was das
   Pane an Feldern oder Aktionen mehr konnte als die Route, zieht als Reiter
   oder Kebab-Eintrag auf die Route um.

3. **„Neu anlegen" steht immer als Kopf-Aktion**, auf jeder Sammlung ihres
   Typs, mit demselben Wort („Kind anlegen", „Raum anlegen"). Es gibt keine
   Sammlung, auf der man das Objekt sehen, aber nicht anlegen kann, während
   es an anderer Stelle anlegbar ist.
4. **Zeilenaktionen ausschließlich im Kebab der Zeile.** Keine Icon-Reihe,
   keine Aktion, die nur beim Überfahren erscheint. Was das Objekt betrifft
   und nicht die Liste, gehört in die Objektansicht. Auch die eine „wichtige"
   Aktion („Veröffentlichen" eines Entwurfs, „Elternlink kopieren") ist ein
   Menüeintrag und kein Knopf neben dem Kebab; sie steht dann oben im Menü,
   und ein Ergebnis, das der Knopf sonst anzeigen würde (Häkchen „kopiert"),
   meldet ein Toast (#3111). Umsortieren (Pfeile ↑/↓) ist eine Listenaktion
   und bleibt sichtbar; das X an einem Chip in einem Formular entfernt einen
   Eingabewert und ist keine Zeilenaktion.
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
9. **Stammdaten der Schule werden auf einer Route verwaltet, nie in einem
   Overlay und nie in einem Auswahlfeld.** Eine Liste, die man anlegen,
   umbenennen, umsortieren, archivieren oder löschen kann, ist eine Sammlung
   (Bauart 1) mit einer Objektansicht (Bauart 2) in der Datenverwaltung —
   auch dann, wenn sie kurz ist und nur ein Formular sie braucht. Ein
   Slide-over mit `view: list | form`, ein Popover mit
   `select | manage | form` und ein Auswahlfeld mit Stift- und
   Deaktivieren-Symbolen an den Zeilen sind dieselbe fünfte Bauart, die es
   nicht gibt; aus einem Formular geöffnet stapeln zwei davon zusätzlich
   Ebenen (Bauart 2 Regel 3).

   **Was ein Formular stattdessen trägt:** die Auswahl, und daneben einen
   Link „<Stammdaten> verwalten" auf die Route. Der Link öffnet einen neuen
   Tab, wenn er in einem Formular steht: der halb ausgefüllte Entwurf lebt
   nur im Zustand des Formulars, ein Wechsel im selben Tab wirft ihn weg.
   Beim Zurückkommen lädt die Auswahl ihre Einträge neu (`focus`), sodass
   der eben angelegte Eintrag ohne Zutun dasteht. Eine Kopf-Aktion einer
   Seite („Schichtarten verwalten") hat keinen Entwurf zu verlieren und
   führt im selben Tab auf die Route.

   Nicht gemeint sind die Einträge des Objekts, um das der Dialog ohnehin
   geht (Teilentschuldigungen eines Kindes, Erziehungsberechtigte an einem
   Kind): die gehören ans Objekt (Bauart 2 Regel 4).

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
   nicht. Der Fuß des Bearbeiten-Zustands ist `ui/EditActions` (Abbrechen +
   Speichern, in dieser Reihenfolge, mit Speichert-Zustand); ein Formular
   ohne dieses Bauteil hat kein Speichern (#3112). Ein Feld, das bei Blur
   oder im Change-Handler schreibt, ist Auto-Save, auch wenn daneben ein
   Speichern für ein anderes Feld steht (Zahlungskonto vor #3112). Das
   Nachtragen eines Objekts in eine Liste (Kind in die Aufsicht, Person an
   ein Kind) ist eine Kopf-Aktion mit `FormModal`, kein Formular im
   Listenkörper.

   **Slide-over oder `FormModal` (#3115):** `FormModal` ist der Standard
   für Anlegen und Nachtragen. Ein Slide-over nur, wenn das Formular
   breiter ist, als ein Modal trägt (mehrstufiger Assistent mit
   Empfängerwahl), oder die Liste dahinter sichtbar bleiben muss. Ein
   Slide-over ist nie eine Detailansicht (Bauart 1 Regel 2). Ein Assistent
   mit zwei Abschlüssen („Als Entwurf speichern" und „Veröffentlichen")
   behält seinen eigenen Fuß; `EditActions` gilt für den Bearbeiten-Zustand
   mit genau einem Speichern.

5. **Fehler stehen im `Alert` oben im Bearbeiten-Bereich und, wo zuordenbar,
   am Feld.** Kein Toast als einzige Fehlermeldung, kein roter Absatz.
   Der Platz dafür ist fest: `error` an `FormModal`, `error` an
   `SlideOverBody`, sonst `FormErrorAlert` als erstes Element des
   Formulars; das Feld trägt seinen Fehler über das `error`-Prop des
   Kit-Felds. Der Fehler-Zustand kommt aus `useFormError()`: der Alert
   scrollt sich bei jedem fehlgeschlagenen Speichern in den sichtbaren
   Bereich, auch beim zweiten Klick mit gleichem Text, weil ein langes
   Formular beim Speichern meist am Fuß steht.
   Ein Fehler-Toast aus dem Speichern-Handler entfällt ganz, auch neben
   einem Alert: eine Meldung, an einem Ort (#3113). Erfolgs-Toasts und
   Toasts für Aktionen ohne Formular (Löschen, Umschalten, Laden) bleiben.
6. **Löschen ist portalweit ein Muster:** `ConfirmDeleteModal`. Die
   Texteingabe-Bestätigung ist die Stufe für Unwiderrufliches mit
   Datenverlust, sonst reicht die einfache Rückfrage. Kein `window.confirm`,
   kein Umschalten des Formular-Footers, kein eigenes Löschmodal je Domäne,
   kein `ConfirmationModal` für Löschen. Braucht eine Löschung eine
   Reichweite (Serie/Einzeltermin, nur dieses Kind/vollständig), liegt die
   Wahl im `scope`-Slot des Bauteils, nicht in einer vorgeschalteten
   `ChoiceModal`; die Wahl ist dann der erste Schritt der Rückfrage.
   Zustandswechsel mit Entfernen-Folge (Stornieren, Widerrufen, Abmelden,
   Archivieren, „Änderung speichern“) sind keine Löschung und bleiben auf
   `ConfirmationModal` (#3110). **Die Rückfrage kommt immer vor der
   Aktion:** ein Knopf oder Menüeintrag, der etwas löscht, entfernt,
   archiviert, zurücknimmt, widerruft oder storniert, öffnet den Dialog;
   die API läuft in dessen `onConfirm`, nie direkt aus dem Klick (#3109).
   Ein Rückgängig-Toast ersetzt die Rückfrage nicht. Dieser Absatz gilt
   abweichend vom Kopf dieses Dokuments in allen Portalen; im Eltern-Portal
   trägt `ConfirmDeleteModal` übersetzte Beschriftungen (`cancelLabel`,
   `confirmLabel`, `closeLabel`) und `mobileSheet`.
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
   Objekttyp. **Umgesetzt** (`scripts/oxlint-plugin-bauart.mjs`, hard-zero,
   #3115): `MasterDetailLayout` darf nur von den Typen importiert werden,
   deren Pane die einzige Objektansicht ist (feste Liste im Plugin: Gruppe,
   Gerät, Rolle, Aktivität, Berechtigung, Kataloge); `DetailPanel` und
   `DatabaseDetailHeader` in einem Overlay sowie `InfoSection`/`DataGrid`
   in einem `SlideOver`/`Drawer` fallen durch. Zentrierte Modale bleiben
   außen vor, weil ein `FormModal` ein Feld zur Einordnung zeigen darf;
   dasselbe gilt ortsgebunden für ein Formular im Slide-over (Übergabe einer
   Gruppe). Operator-, Eltern- und Schul-Portal sind nicht im Scope.
2. `bauart/no-local-field-grid` — kein lokales `<dt>/<dd>`-Feldgitter
   außerhalb `ui/detail-modal-components`.
3. `bauart/one-delete-confirm` — nur `ConfirmDeleteModal`; kein
   `window.confirm`, kein `ConfirmationModal` für Löschen. **Umgesetzt**
   (`scripts/oxlint-plugin-bauart.mjs`, hard-zero, #3110): prüft die
   Beschriftung der Hauptaktion (`confirmText` einer `ConfirmationModal`,
   `title` einer `Modal`/`ChoiceModal`/`FormModal`) auf „löschen“ /
   „entfernen“ und jeden `window.confirm`-Aufruf.
4. `bauart/no-own-skeleton` — kein eigenes Seiten-Skelett neben den
   `TenantPage`-Zuständen.
5. `bauart/no-raw-status-hex` — keine rohen Hexwerte für Status- und
   Planungsfarben.
6. `bauart/no-disabled-menu-item` — keine dauerhaft deaktivierten
   Menüeinträge.
7. Erweiterung von `tenant-page-scaffold.test.ts`: jede Seite deklariert ihre
   Bauart, und die Zuordnung ist vollständig.
8. `bauart/no-unconfirmed-destructive-click` — kein Löschen, Entfernen,
   Archivieren, Zurücknehmen, Widerrufen oder Stornieren direkt aus dem
   Klick. **Umgesetzt** (`scripts/oxlint-plugin-bauart.mjs`, hard-zero,
   #3109): prüft `Button`/`button` und Menüeinträge (`label` + `onClick`)
   mit einer solchen Beschriftung darauf, ob der Inline-Handler selbst eine
   asynchrone Aktion auslöst (`void f()`, `await f()`, `.then(`); ein
   Klick, der nur den Dialog öffnet (`setDeleteTarget(x)`), passiert. Ein
   per Namen übergebener Handler (`onClick={handleDelete}`) liegt außerhalb
   der Ratsche und gehört ins Review.
9. `bauart/no-row-action-buttons` — Zeilenaktionen nur im Kebab (Bauart 1
   Regel 4). **Umgesetzt** (`scripts/oxlint-plugin-bauart.mjs`, #3111): ein
   `Button`/`<button>` je Listeneintrag (in einem `.map(…)` oder dem
   `render` einer Tabellenspalte), dessen zugänglicher Name eine
   Objektaktion ist (bearbeiten, löschen, entfernen, archivieren,
   wiederherstellen, duplizieren, umbenennen, veröffentlichen, kopieren),
   fällt durch. Pfeile zum Umsortieren und das reine Icon-X „… entfernen"
   eines Formular-Chips sind ausgenommen. Shrink-only Baseline je Datei für
   den Bestand, den #3111 auf Folge-PRs verteilt; Operator-, Eltern- und
   Schul-Portal sind nicht im Scope.
10. `bauart/no-autosave` — kein Schreiben aus `onBlur` und keines aus dem
    Change-Handler eines Formularfelds ohne Speichern darunter. **Umgesetzt**
    (`scripts/oxlint-plugin-bauart.mjs`, #3112): jeder Inline-`onBlur`, der
    einen asynchronen Aufruf feuert (`void save(x)`, `await update(x)`,
    `.then(`), und jeder Inline-Change-Handler an einem Kit-Feld
    (`Input`, `Textarea`, `Checkbox`, `CustomSelect`, `ListboxDropdown`,
    `SegmentedControl`, …), dessen gefeuerter Aufruf wie ein Schreiben heißt
    (save, update, persist, patch, set…, submit, store, assign, link, mutate).
    Lesen aus dem Change-Handler (`void search(q)`) passiert. Ausgenommen sind
    die Einstellungen (Bauart 4), das Operator-Portal, das Kit, Tests und
    Stories; die benannte Baseline im Plugin ist shrink-only und trägt nur
    Flächen, die ihr Sofort-Speichern auf dem Schirm benennen (Abrechnung,
    Elternportal-Stammdaten, Sprachwahl).
11. `bauart/no-toast-form-error` — kein Fehler-Toast aus dem
    Speichern-Handler eines Formulars (Bauart 2 Regel 5). **Umgesetzt**
    (`scripts/oxlint-plugin-bauart.mjs`, hard-zero, #3113): ein
    `toast.error`/`toast.warning` (auch die Aliasse `toastError`,
    `toastWarning`) innerhalb einer Funktion, die ein Formular speichert,
    fällt durch. Als Speichern-Handler gilt eine Funktion, die
    `handleSave`, `handleSubmit`, `onSubmit`, `save…` oder `submit…` heißt,
    einen `FormEvent`-Parameter hat oder selbst `preventDefault()` ruft;
    Rückrufe darin (`.catch(() => toast.error(…))`) zählen mit. Erfolgs-
    Toasts und Toasts außerhalb solcher Handler sind frei. Operator-,
    Eltern- und Schul-Portal sind nicht im Scope.
12. `bauart/no-manage-surface-in-overlay` — keine Verwaltungsfläche für
    Stammdaten in einem Overlay oder Auswahlfeld (Bauart 1 Regel 9).
    **Umgesetzt** (`scripts/oxlint-plugin-bauart.mjs`, hard-zero, #3114):
    ein Kebab-Eintrag oder Knopf je Zeile, dessen Beschriftung eine
    Objektaktion ist (bearbeiten, löschen, archivieren, wiederherstellen,
    umbenennen, duplizieren, (de)aktivieren), fällt durch, sobald er in
    `SlideOver`, `Modal`, `FormModal`, `Drawer`, `AnchoredPopover` oder in
    einem Menü-Slot von `ListboxDropdown` landet — auch über eine
    Hilfsfunktion, die eine dieser Hüllen rendert. „entfernen" steht nicht
    in der Liste, weil es in einem Dialog meist einen Formularwert
    entfernt; dafür gilt `bauart/no-row-action-buttons`. Die ortsgebundenen
    Ausnahmen sind Einträge des Objekts, das der Dialog bearbeitet.
    Operator-, Eltern- und Schul-Portal sind nicht im Scope.

## Reihenfolge der Umsetzung

1. **Rahmen fertig** — die verbleibenden Seiten ohne `TenantPage` nachziehen,
   die sechs eigenen Ladeskelette auf die Gerüst-Zustände umstellen,
   `error`/`empty` korrekt belegen.
2. **Bauart 2 zuerst** — eine Objektansicht je Typ, die Doppelbäume von Kind,
   Mitarbeiter und Raum zusammenführen, Verwaltungsfelder als Reiter. Das ist
   zugleich die halbe Navigationsentscheidung.
3. **Wörter** — Wörterbuch mit Ratsche, Weiterleitungen für alte Pfade,
   Hilfe-Anleitung im selben Zug. (Der Navigationsumbau aus Teil 2 ist am
   29.08.2026 zurückgenommen worden — siehe dort.)
4. **Bauart 1 und 3** — Sammlungen und Werkzeuge angleichen.
5. **Farbe und Ruhe** — einfarbige Navigation, Statusfarben aufräumen.
6. **Selbsterklärung** — jeder Leerzustand als nächster Schritt, jede
   Voraussetzung am Schalter.

Die Ratsche einer Stufe wird mit der Stufe eingeführt, nicht danach.

---

# Teil 2 — Navigation und Wörter

Nachtrag vom 28.08.2026, **zurückgenommen am 29.08.2026.**

## Was zurückgenommen wurde

Teil 2 sah vor, den Bereich „Datenverwaltung" aufzulösen und seine Register
als Reiter an die Sammlungen zu hängen („ein Objekt, ein Ort"). Das ist
gebaut und wieder entfernt worden. Die Entscheidung steht: **die
Datenverwaltung bleibt ein eigener Bereich in der Seitenleiste, genau wie
zuvor.**

Der Grund ist nicht die Idee, sondern ihr Ergebnis auf dem Schirm. Die
Register wanderten als Reiter an vier Sammlungen und in die Einstellungen.
Damit stand über den Kindern ein Reiter „Stammdaten", über den
Mitarbeitenden noch einer, und weil je Fläche höchstens vier Reiter erlaubt
waren, bündelte sie ein Reiter „Verwaltung" — ein Wort, das nicht sagt, was
darin liegt, und das den Wortstamm der „Datenverwaltung" ein zweites Mal
belegt. Eine Verwaltungsfläche war damit an fünf Stellen erreichbar und an
keiner benannt. Ein zusammenhängender Bereich mit zehn klaren Namen ist
verständlicher als zehn Namen, die über die Seiten verteilt und hinter einem
Sammelwort versteckt sind.

Was aus Teil 2 gültig bleibt, weil es unabhängig davon steht:

- **Die Wörter** (Tabelle unten) — ein Begriff, ein Wort, überall gleich.
- **Kein geteilter Wortstamm** ohne sichtbare Abgrenzung.
- **Einfarbige Navigation** — Farbe ist Status, kein Bereichsschmuck.
- **Nichts läuft ins Leere** — jede alte Route bleibt als Weiterleitung.

Nicht mehr gültig: die Seitenleiste auf neun Bereiche, das Auflösen der
Datenverwaltung, die Register als Reiter an den Sammlungen, der Sammelreiter
„Verwaltung" (siehe Teil 3, Regel 3).

## Die Wörter

Ein Begriff, ein Wort, überall gleich: Seitenleiste, Brotkrume, Seitentitel,
Knopf, Hilfe, E-Mail. Verbindlich:

| Statt                                     | Ab jetzt                              |
| ----------------------------------------- | ------------------------------------- |
| „Alle Kinder" / „Kindersuche" / „Kinder"  | **Kinder**                            |
| „Mitarbeiter" / „Personal" / „Lehrkräfte" | **Mitarbeitende**                     |
| „Betreuungsangebote"                      | **Angebote** (im Bereich Anmeldungen) |
| „Tagesauswertung"                         | **Tagesbericht**                      |

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

**Der Grund endet nie mitten im Bild.** Der Rumpf einer Seite füllt die Höhe
des Bildschirms; seine letzte Fläche wächst bis zur Unterkante
(TENANT-PAGE-SPEC, Regel 6). Eine leere Prüfliste ist damit dieselbe weiße
Fläche wie eine volle, nicht eine 60-px-Zeile über einem halben Bildschirm
Punktraster.

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

- _Abstände._ Trägt nur der aktive Reiter einen Strich, richtet sich die Zeile
  an etwas aus, das den übrigen fehlt: unter deren Text steht fast doppelt so
  viel Luft. Mit der Linie am Band sind alle Reiter gleich hohe Kästen.
- _Verständlichkeit._ Die Linie verbindet den Reiter sichtbar mit dem Inhalt
  darunter. Eine getönte Pille oder eine geschlossene Segment-Spur sagt das
  nicht — beide lesen sich als „wähle einen Wert", nicht als „wechsle die
  Ansicht". Genau daran scheiterten die ersten beiden Fassungen.

Kein `-mb-px` an den Reitern. Der Reiter „Mehr" rendert eine zusätzliche
Hülle um seine Schaltfläche; ein negativer Rand greift dort an der inneren
Schaltfläche und verschiebt genau diesen einen Reiter um ein Pixel.

## 3. Reiter werden gemessen, nicht gebündelt

Nachtrag vom 29.08.2026, ersetzt die frühere Regel „höchstens vier
Seitenreiter": Seiten bündeln ihre Reiter NICHT von Hand. Eine Seite übergibt
alle Reiter flach an `TenantPage`; das Gerüst misst, wie viele in die Zeile
passen, und räumt nur den Überhang in einen letzten Reiter **„Mehr"** mit
Menü. Passt alles, gibt es kein „Mehr".

Warum die alte Regel weg ist: ein benannter Sammelreiter („Verwaltung") ist
geraten. Er verrät nicht, was in ihm liegt, er bündelt auch dann, wenn der
Platz längst reicht, und er trug denselben Wortstamm wie der Bereich
„Datenverwaltung" — genau die Dublette, die diese Spezifikation verbietet.

Die Messung läuft über eine unsichtbare Schattenzeile mit allen Reitern in
natürlicher Breite (`ResizeObserver` auf der sichtbaren Zeile). Ohne Messwerte
— Testumgebung, noch nicht gelayoutet, Zeile verborgen — stehen ALLE Reiter
da. Ein „Mehr" zu bauen, weil die Breite unbekannt ist, versteckt Bereiche
ohne Grund.

Das Menü hinter „Mehr" ist innen abgesetzt: die Zeilen sind gerundet und
haben Abstand zur Kante des Kastens. Eine eckige Hover-Fläche in einem
gerundeten Kasten liest sich als Fehler.

## 4. Farbe bleibt Status

Unverändert gültig (Querregel Farbe). Ergänzend aus der Sichtprüfung: eine
Kennzahl, die nur groß ist, ist kein Warnzustand. Orange und Rot an einer Zahl
bedeuten, dass jemand handeln muss — sonst bleibt sie neutral.

## 5. Kennzahlen: erst die eine Antwort, dann das Detail

Eine Reihe gleichförmiger Kennzahl-Kacheln ist eine Tabelle ohne Kopfzeile —
sieben Werte nebeneinander liest niemand als Aussage (gemessen auf
/statistics). Das Eltern-Portal-Muster gilt auch hier: EINE Zahl ist die
Antwort der Fläche und steht groß (`text-2xl`), zwei bis drei Nebenwerte
stehen kleiner daneben, alles Weitere gehört in die Tabelle oder Liste
darunter. Werte, die schon in der Statuszeile der Kopfkarte stehen, tauchen
in keiner Kachelreihe erneut auf — die vier Kacheln der Startseite
wiederholen sich auf keiner Unterseite.
