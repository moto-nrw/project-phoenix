# Diff einer entschiedenen Änderungsanfrage wird beim Entscheiden eingefroren

Der Diff einer Angebots-Änderungsanfrage wurde bisher bei jedem Lesen aus dem
Eltern-Payload und den *aktuellen* Mitbuchungs-Regeln neu materialisiert. Nach
einer Regeländerung (real: OGS am Berg löschte ihre Regeln am 17.08.2026)
zeigte der Recap einer alten Anfrage damit etwas anderes als das, was bei der
Freigabe tatsächlich gebucht wurde. Entscheidung: Beim Entscheiden (Freigabe
oder Ablehnung) wird der materialisierte Diff — inklusive Kennzeichnung
automatischer Anteile und etwaiger Übersteuerungen — als Snapshot an der
Anfrage gespeichert; Recap-Ansichten lesen nur noch diesen Snapshot. Offene
Anfragen werden weiterhin live materialisiert, damit sie Regeländerungen vor
der Entscheidung korrekt abbilden.

## Consequences

- Historie ist unveränderlich: Regel- oder Angebots-Änderungen nach der
  Entscheidung verändern entschiedene Anfragen nicht mehr.
- Der Snapshot ist zugleich der Speicherort für Übersteuerungen (#2370);
  ohne ihn wäre "welche Regel wurde von wem abgewählt" nicht rekonstruierbar.
- Bestandsanfragen ohne Snapshot brauchen einen Lese-Fallback auf die bisherige
  Live-Materialisierung.
