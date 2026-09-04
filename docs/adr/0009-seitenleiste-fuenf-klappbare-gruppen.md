# Seitenleiste: fünf klappbare Gruppen, ein Baum für Desktop und Mobil

Status: accepted (#2826). Ergänzt ADR 0008 (Datenverwaltung getrennt von
Alle Kinder).

## Kontext

Die Mitarbeiter-Seitenleiste war eine flache Liste von rund 20 Einträgen
ohne erkennbare Ordnung: drei ad-hoc-Filter plus sieben hart codierte
Akkordeons, deren Reihenfolge im Code stand und nirgends sonst. Mobil hatte
das Mehr-Menü eine eigene Liste mit eigenen Namen („Suchen" statt „Alle
Kinder", „Gruppe" statt „Meine Gruppen"), und drei Seiten fehlten dort ganz
(Tagesauswertung, Dateien, Info-Displays). Das Issue verlangt: fachlich
gruppieren und stärker verschachteln, Reihenfolge nach Nutzungshäufigkeit,
Mobile aus derselben Struktur, und eine Entscheidung zu Datenverwaltung
gegen Alle Kinder (ADR 0008).

Ein erster Anlauf (Branch `feat/2826-sidebar-structure`, nicht gemergt)
probierte vier Varianten: Überschriften über derselben Liste, ein
persönlicher Schnellzugriff, alles in Akkordeons, Häufigkeitsstufen mit
Trennlinien. Keine überzeugte; die Erkenntnisse daraus sind unten
eingearbeitet.

## Entscheidung

Die Seitenleiste besteht aus **fünf klappbaren Gruppen**, für alle Rollen
dieselbe Liste; was eine Person nicht sehen darf, fällt weg, leere Gruppen
ebenso. Über den Gruppen steht die Startseite der Rolle (Home bzw.
Tagesplan), darunter angeheftet Notfall, Hilfe und Einstellungen.

| Gruppe | Zeilen, in dieser Reihenfolge |
|---|---|
| **Tagesbetrieb** | Meine Gruppen (mit Weitere Gruppen), Aktuelle Aufsicht, Alle Kinder, Räume, Aktivitäten, Vertretungen, Anfragen |
| **Eltern** | Nachrichten, Mitteilungen, Elternzugänge, Bankverbindungen, Essensplan, Anmeldungen |
| **Team** | Zeiterfassung, Mein Kalender, Mitarbeiter, Team-Chat, Tagesinformationen |
| **Planung** | Betreuungsplan, Dienstplan, Vertretungsplan, Tageslisten, Schuljahr und Ferien, Abrechnung |
| **Verwaltung** | Datenverwaltung, Tagesauswertung, Statistik, Dateien, Info-Displays |

Regeln:

1. **Die Gruppenzeile ist ein Schalter, keine Seite.** Sie klappt die
   Gruppe auf und zu und führt nirgendwohin. Das frühere Navigate-on-expand
   der Akkordeons Eltern, Kommunikation und Planung entfällt mit ihnen.
2. **Beim ersten Besuch steht nur der Tagesbetrieb offen.** Was die Person
   auf- oder zuklappt, merkt sich der Browser (`sidebar-open-groups`). Die
   Gruppe der aktuellen Seite öffnet sich von selbst, ohne die anderen zu
   schließen; ein Seitenwechsel schließt nichts. Kinder-Detailseiten gehören
   zu der Gruppe, aus der man gekommen ist (`?from=`), sonst zum
   Tagesbetrieb.
3. **Akkordeons gibt es nur noch für echte Einheiten mit eigenen
   Unterpunkten:** Meine Gruppen und Aktuelle Aufsicht (aus der Sitzung),
   Datenverwaltung und Anmeldungen (Hub-Seite mit festen Unterseiten). Sie
   bleiben exklusiv (einer offen) und behalten ihr Navigate-on-expand.
   Eltern, Kommunikation und Planung waren Akkordeons für erfundene
   Oberbegriffe; ihre Seiten stehen jetzt als Zeilen mit Icon direkt in
   ihrer Gruppe.
4. **Reihenfolge innerhalb einer Gruppe = Häufigkeit im OGS-Alltag.** Die
   Einschätzung stammt aus den Arbeitsabläufen, nicht aus Nutzungsdaten;
   PostHog zählt Seitenaufrufe, sobald jemand nachsieht, gehört die Liste
   dagegen geprüft.
5. **Anfragen steht im Tagesbetrieb, nicht bei Eltern:** das Modul bündelt
   seit #2429 auch die Anträge von Mitarbeitenden (Urlaub, Krank,
   Fortbildung) und trägt den einzigen Zähler auf oberster Ebene; im immer
   offenen Tagesbetrieb bleibt er sichtbar.
6. **Der eingeklappte Streifen zeigt dieselben Gruppen.** Die Gruppenzeile
   wird dort zum Icon und klappt nur, ohne die Leiste zu öffnen; die Zeilen
   einer offenen Gruppe passen als Icons in 64 px. Die Akkordeons verhalten
   sich wie bisher (Klick öffnet die Leiste). Die Gruppen-Icons sind keines
   der Seiten-Icons darunter, sonst stünden zwei gleiche Bilder
   untereinander.
7. **Mobil ist das Mehr-Menü dieselbe Liste:** Startseite oben, dann die
   fünf Gruppen mit Überschrift, unten Notfall, Hilfe und Einstellungen. Die
   Reiter unten bleiben die festen Rollenlisten; was sie schon zeigen, fehlt
   im Menü. Datenverwaltung und Anmeldungen sind dort eine Zeile auf den
   Hub, Meine Gruppen und Aktuelle Aufsicht eine Zeile auf ihre Übersicht.
8. **Umbenannt, weil zwei Einträge denselben Wortstamm trugen:**
   Planung › Terminvertretungen heißt **Vertretungsplan** (wie
   Betreuungsplan und Dienstplan daneben; die Endung zeigt die Grenze zur
   Tagesübersicht „Vertretungen" im Tagesbetrieb), Planung ›
   Kalenderzeiträume heißt **Schuljahr und Ferien** (die Seite verwaltet
   Schuljahr, Halbjahre, Ferien und Schließtage; der Fachbegriff bleibt im
   Formularfeld, das einen Zeitraum auswählt), Eltern › Konto-Anfragen
   heißt **Elternzugänge** (neben dem Modul „Anfragen"), Datenverwaltung ›
   Kinder heißt **Kinderdaten** (ADR 0008), Eltern › Mitteilungen und
   Umfragen heißt **Mitteilungen** (der lange Name passte eingerückt nicht
   mehr in die Zeile; die Seite selbst zeigt Mitteilungen, Elternbriefe und
   Umfragen als Reiter). Die Objektbegriffe auf den Seiten selbst
   („Terminvertretungen" auf der Vertretungen-Tagesseite,
   „Kalenderzeitraum" im Formular) bleiben.
9. **Die Hub-Seite /eltern ist entfallen.** Ihre Kacheln waren nur ein
   zweiter Weg zu Seiten, die jetzt direkt in der Gruppe stehen. Die Route
   /eltern/bankverbindungen bleibt. Die Breadcrumb zeigt „Eltern › Seite"
   und „Team › Seite" als Text ohne Link, wie schon bei Planung.

## Verworfen

- **Feste Überschriften ohne Klappen** (alles sichtbar): ein Admin mit
  allen Modulen käme auf rund 30 Zeilen und rollte auf 900 px Höhe schon im
  Grundzustand. Die Überschriften-Variante des ersten Anlaufs machte die
  Leiste länger, nicht kürzer.
- **Nur Trennlinien nach Häufigkeit** (der erste Anlauf): die Blöcke hatten
  keinen Namen, und ein Dropdown „Eltern" neben einem Dropdown
  „Kommunikation" blieb.
- **Persönlicher Schnellzugriff**: Personalisierung kaschiert eine
  schlechte Struktur, statt sie zu beheben.
- **Ein Bereich „Betreuung"** für Räume, Aktivitäten, Vertretungen: vierter
  Eintrag mit dem Wortstamm neben Betreuungsplan, Betreuungsangebote und
  Betreuungszeiten (`.claude/rules/verstaendlichkeit.md`, das Muster aus
  #2295).

## Konsequenzen

- **Ein Baum, zwei Konsumenten.** `frontend/src/lib/staff-navigation.ts`
  ist die einzige Quelle für Gruppe und Reihenfolge; Seitenleiste
  (`sidebar.tsx`) und Mehr-Menü (`mobile-bottom-nav.tsx`) rendern daraus.
  Namen und Pfade kommen weiterhin aus den Katalogen
  (`section-navigation.ts`, `planning-navigation.ts`), die
  Sichtbarkeitsregeln bleiben in den beiden Komponenten. Wer eine Seite in
  die Navigation aufnimmt, trägt sie in genau eine Gruppe ein;
  `staff-navigation.test.ts` schlägt an, wenn eine Katalogseite nirgends
  oder doppelt steht.
- Der Gruppenzustand lebt in `use-sidebar-groups.ts`
  (`sidebar-open-groups`), der Akkordeon-Zustand weiter in
  `use-sidebar-accordion.ts` (`sidebar-accordion-expanded`, alte Werte
  `planning`/`eltern`/`kommunikation` werden ignoriert).
- Die Gruppenzeile ist `SidebarGroup` (`sidebar-group.tsx`), 32 px hoch
  auf derselben Icon-Achse wie die Zeilen (`SIDEBAR_GROUP_HEADING_CLASSES`
  in `sidebar-geometry.ts`). Die Zeilen einer Gruppe stehen ausgeklappt
  16 px eingerückt hinter einer Linie unter dem Gruppen-Icon: ohne die
  Einrückung las sich die Leiste weiter als flache Liste mit
  Zwischenüberschriften. Im Streifen gleitet die Einrückung mit der Breite
  auf null.
- Die Breadcrumb-Sektion „Kommunikation" heißt „Team", damit Leiste und
  Kopfzeile dasselbe Wort zeigen.
- Die Hilfe beschreibt die Seitenleiste in einem eigenen Schritt und nennt
  die neuen Bezeichnungen.
