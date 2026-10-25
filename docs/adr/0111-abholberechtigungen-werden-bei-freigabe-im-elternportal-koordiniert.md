---
status: accepted
---

# Abholberechtigungen werden bei Freigabe im Elternportal koordiniert

Entscheidung vom 25.10.2026 für #4780 (ADR 0111).

## Entscheidung

Die Freigabe einer unterschriebenen Abholberechtigung und einer
Tagesabholung prüft die Kontaktperson erst im Augenblick der OGS-Entscheidung
erneut. Der Parent-Portal-Workflow koordiniert diese kurze Folge in der
bereits geöffneten Mandanten-Transaktion:

1. Er sperrt das Guardian-Profil und danach die Beziehung zum Kind.
2. Er liest beide Werte unter den Sperren erneut und lehnt Kontoinhaber,
   vollberechtigte Sorgeberechtigte, Sozialarbeiter und entfernte Kontakte ab.
3. Bei einer Wochenberechtigung ruft er den Care-Plan-Befehl zum Schreiben der
   Abholberechtigung auf. Bei einer Tagesabholung bleibt die Sperre bis Care
   Plan die datierte Abholnotiz geschrieben hat.

Der Workflow beschreibt dafür eine eigene Schreibschnittstelle. Die Root
übersetzt sie auf Care Plans öffentliche Berechtigungs-Schnittstelle. Weder
Parent Portal noch Care Plan importieren den anderen Owner.

## Folgen

Dies ist eine eng begrenzte Koordinationsausnahme nach ADR 0067: Sie umfasst
nur die beiden genannten Freigaben und keine allgemeine Care-Plan-zu-
Parent-Portal-Abhängigkeit. Der Effektkatalog führt die Bindungen als Schuld,
bis die Freigabe-Schnittstelle in einen eigenständigen, von Care Plan nicht
aufgerufenen Workflow-Einstieg geschnitten werden kann. Fehler beim erneuten
Prüfen bleiben für Care Plan `pickup permission contact is no longer linked`;
ein abgelehnter oder geänderter Kontakt erzeugt keine Berechtigung.
