---
status: proposed
---

# Buchungsgebundene Planung projiziert zukünftige Teilnehmer zur Lesezeit

Der Produktionsvorfall bei der OGS am Berg im August 2026 zeigte, dass
Angebotsbuchungen, Bring- und Gehzeiten sowie Termin-Teilnehmerlisten aus
verschiedenen, teils undatierten Kopien gelesen werden. Ereignisgetriebene
Abgleiche hielten diese Kopien nicht zuverlässig zusammen. Wir führen deshalb
eine buchungsgebundene Teilnehmerplanung ein, die den zukünftigen Sollstand aus
wirksamen fachlichen Quellen berechnet und erst beim operativen Beginn eines
Termins als historischen Stand festhält.

## Entscheidung

### Betriebsprofile statt eines globalen autoritativen Modus

Schulen kombinieren Anmeldung, Anwesenheit, Betreuungsplan sowie Dienst- und
Zeiterfassung über einen geführten Einrichtungsprozess. Für die
Teilnehmerplanung gibt es die Zustände `manual`, `shadow` und `bookings`:

- `manual` verwendet weiterhin manuell gepflegte wiederkehrende Kinderlisten.
- `shadow` berechnet die buchungsgebundene Planung parallel, zeigt Unterschiede
  und verändert den operativen Plan noch nicht.
- `bookings` verwendet den berechneten Sollstand für die operative Planung.

Der Zustand ist eine mandantenbezogene Konfiguration im Settings-System, aber
kein frei editierbares Standardfeld. Ein eigener Einrichtungs- und
Umstellungsflow prüft Voraussetzungen und erlaubt nur gültige Übergänge. Neue
Schulen können nach erfolgreicher Vorprüfung im Onboarding direkt mit
buchungsgebundener Planung beginnen. Bestandskunden bleiben unverändert, bis
sie ausdrücklich wechseln.

### Fachliche Quellen des Betreuungsfensters

An Schultagen beginnt das Betreuungsfenster eines Kindes mit dem datierten
Unterrichtsschluss seiner Schulklasse. Es endet mit der spätesten Endzeit aller
an diesem Datum wirksamen, betreuungswirksamen Angebotsbuchungen. Ohne eine
solche Buchung gibt es in der buchungsgebundenen Planung an diesem Wochentag
keinen Betreuungstag.

Jedes betreuungswirksame Angebot braucht an jedem wählbaren Wochentag eine
Endzeit. Unterrichtsschluss und Angebots-Endzeiten werden mit Gültigkeitsfenstern
geändert; eine Änderung schreibt frühere Betreuungsfenster nicht um. Eine
datierte Kind-Überschreibung kann Beginn oder Ende höchstens bis zum Ende des
zugehörigen Halbjahres ersetzen. Eine Tagesausnahme gilt weiterhin nur für ihr
Datum.

Der Projektor liest ausschließlich wirksame Live-Daten. Offene Anfragen ändern
weder Zeiten noch Teilnehmer. Die bestehenden Freigabeprozesse bleiben
unverändert.

### Eine sichtbare Teilnehmerregel pro wiederkehrendem Termin

In der buchungsgebundenen Planung erhält jede wiederkehrende Vorlage ihre
Kinder durch genau eine sichtbare Teilnehmerregel. Die zunächst unterstützte
Regelsprache besteht aus:

1. einem oder mehreren Betreuungsangeboten als deduplizierte Union,
2. optional genau einem Jahrgangs- oder Klassenfilter und
3. dem Wochentag des konkreten Termins, der die gebuchten Wochentage schneidet.

Eine allgemeine Regel-Engine, freie Prädikate, Prioritäten und Fallbacks gehören
nicht zu dieser Entscheidung. Dauerhafte manuelle Kinderlisten sind nur in der
manuellen Planung erlaubt. Die buchungsgebundene Planung erlaubt stattdessen
begründete einmalige Inklusionen und Exklusionen an einem konkreten Termin.
Eine ungeplante Betreuung erzeugt keine Angebotsbuchung und keine Dauerregel.

### Dynamischer Sollstand, eingefrorener operativer Stand

Termininstanzen bleiben materialisiert, weil Raum, Zeit, Aufsicht, Status und
Einzeltag-Abweichungen eine stabile Termin-ID benötigen. Der Teilnehmer-Sollstand
eines zukünftigen, vorlagengebundenen und rein geplanten Termins wird dagegen
zur Lesezeit in einer zentralen Batch-Projektion berechnet. Wochenplan,
Tageslisten, Exporte, Kapazitätsprüfungen und Konfliktanzeigen müssen denselben
Projektor verwenden.

Eine geplante Einzeltag-Inklusion oder -Exklusion liegt als ausdrückliche
Abweichung mit Herkunft und Grund über dem weiterhin dynamischen Sollstand. Der
Teilnehmerstand wird beim frühesten operativen Ereignis atomar als Snapshot
festgehalten: beim Start des Termins, beim ersten Check-in, bei der ersten
manuellen Statusentscheidung, beim Abschluss oder Abbruch oder spätestens beim
Tagesabschluss für einen vergangenen, noch geplanten Termin. Spätere
Quelländerungen verändern operative und historische Snapshots nicht.

### Sichtbare Konsistenzprüfung mit begrenzter Reparatur

Ein wiederholbarer tenantweiter Lauf vergleicht wirksame Quellen und daraus
ableitbare Daten. Im Vergleichsbetrieb liefert er die Umstellungsvorschau; nach
der Umstellung erkennt er fehlende Quellen, ungültige Regeln und unerklärte
Abweichungen.

Der Lauf darf eine Abweichung automatisch und idempotent reparieren, wenn der
Sollwert vollständig feststeht, die betroffenen Daten dem Ableitungsmechanismus
gehören und der Termin noch rein geplant ist. Er verändert niemals
Angebotsbuchungen, Unterrichtsschluss, Angebots-Endzeiten, Teilnehmerregeln,
menschliche Abweichungen oder operative und historische Snapshots. Gefundene,
reparierte und nicht reparierbare Abweichungen sowie Reparaturfehler sind für
OGS-Admins sichtbar; betroffene operative Ansichten zeigen ungelöste Konflikte.
Logs und Metriken ergänzen diese Anzeige, ersetzen sie aber nicht.

### Getrennte Personalplanung

Angebotsbuchungen bestimmen den erwarteten Betreuungsbedarf. Der
Betreuungsplan ordnet Aktivitäten, Räume, Aufsichten und Kinder innerhalb der
Betreuungsfenster ein. Der Dienstplan legt getrennt fest, wann Teammitglieder
arbeiten. Das System zeigt Unterbesetzung und Aufsichten außerhalb ihrer
Schichten als Konflikte, ändert aber keine Schicht aufgrund einer
Buchungsänderung.

## Einführung bei Bestandskunden

Eine bestehende Schule wechselt nur über den Vergleichsbetrieb. Die Vorschau
zeigt mindestens fehlende Quellen, ungültige Buchungen sowie Kinder, die durch
den Wechsel in einen Termin aufgenommen oder daraus entfernt würden. Die OGS
bestätigt die fachlichen Unterschiede und wählt einen Stichtag. Laufende und
historische Snapshots bleiben unverändert.

Die OGS am Berg ist der Pilot. Ihre sechs operativ genutzten manuellen
Randstunden-Serien lassen sich als `Randstunde ∩ Schulklasse ∩ gebuchter
Wochentag` ausdrücken. Der Wechsel erfolgt erst, nachdem die Schule diese Regeln
und die Diff-Vorschau bestätigt sowie fehlende Angebots-Endzeiten und
verdächtige Buchungen geklärt hat.

Der Rückweg zu manueller Planung ist ebenfalls geführt. Zum gewählten Stichtag
wird der dann berechnete Sollstand als datierte manuelle Planung übernommen;
die Vorschau weist darauf hin, dass spätere Buchungsänderungen anschließend
nicht mehr automatisch folgen. Ein direktes Umschalten ist nicht erlaubt.

## Considered Options

- **Globaler Schalter `enrollment.bookings_authoritative`:** verworfen, weil
  Buchungen weder Unterrichtsschluss noch Dienstplan bestimmen und Schulen die
  Module in verschiedenen Kombinationen verwenden.
- **Ereignisgetriebene Synchronisation aller Kopien:** verworfen, weil jeder
  neue Schreibweg sämtliche Ableitungen kennen müsste und zukünftige
  Wirksamkeitsdaten ohne Ereignis erneut driften könnten.
- **Vollständige Materialisierung mit Gültigkeitsfenstern:** bleibt dort
  sinnvoll, wo ein stabiler operativer oder historischer Snapshot nötig ist,
  erzeugt für rein zukünftige Teilnehmerlisten aber unnötige Kopien und
  Abgleichspfade.
- **Alles zur Lesezeit berechnen:** verworfen, weil begonnene und abgeschlossene
  Termine sowie menschliche Entscheidungen einen unveränderlichen historischen
  Stand brauchen.
- **Allgemeine Teilnehmer-Regel-Engine:** verworfen, weil der Pilot nur
  Angebots-Union, Wochentag sowie Klassen- oder Jahrgangsfilter benötigt.

## Consequences

- [ADR 0001](./0001-angebots-gehzeit-materialisieren.md) bleibt für die
  manuelle Planung gültig. Für die buchungsgebundene Planung ersetzt diese ADR
  das Ausrollen von Angebots-Gehzeiten durch die Projektion aus wirksamen
  Angebotsbuchungen.
- Der zentrale Projektor ist die einzige erlaubte Quelle für zukünftige
  Teilnehmer-Sollstände. Ein nur in einzelnen Ansichten eingesetzter Projektor
  würde die heutige Drift lediglich verschieben.
- Die Projektion muss Zeiträume gebündelt laden und darf keine Abfrage pro
  Vorlage, Termin oder Kind ausführen. Die erwarteten Schulgrößen sind klein;
  Produktionsmessungen für Tages- und Mehrwochenansichten entscheiden über
  zusätzliche Indizes.
- Die sofortige Absicherung des heutigen manuellen Betriebs erfolgt über die
  sichtbare Konsistenzprüfung und ihre begrenzte Reparatur, nicht über einen
  weiteren unsichtbaren Sync-Pfad.

## Scope

Diese ADR entscheidet die Planung an regulären Schultagen. Der Beginn eines
Betreuungsfensters in der Ferienbetreuung bleibt offen, bis der operative
Ablauf mit einer Schule geklärt ist. Sie ändert weder bestehende
Freigabeprozesse noch die fachliche Eigenständigkeit des Dienstplans.
