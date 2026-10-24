---
status: accepted
---

# Class Day lässt einen nicht gebundenen Schreibbereich leer

Entscheidung vom 24.10.2026. Sie dokumentiert die Composition-Änderung von
Class Day beim Herauslösen der Plattform- und Domänen-Bindungen. Sie fügt
keinen Owner, keine Policy-Regel und keine Laufzeit-Ausnahme hinzu.

## Kontext

`ClassDayDependencies.WriteScope` ist die optionale Berechtigung zum Schreiben
von Ankunftsausnahmen. Ein nicht gebundener Schreibbereich bedeutet im
Anwendungsdienst ausdrücklich „nicht konfiguriert“. Nach dem Herauslösen der
Bindings muss die Composition dieses Fehlen in den Port
`application.ArrivalWriteScope` übertragen. Eine direkte Zuweisung eines
nil-Interfacewerts kann dabei einen nicht-nil Interfacewert mit nil Payload
erzeugen und die Prüfung auf eine fehlende Konfiguration umgehen.

## Entscheidung

Die Composition setzt den Anwendungsport nur, wenn `deps.WriteScope` gebunden
ist. Andernfalls bleibt der Port nil und der Anwendungsdienst antwortet weiter
mit „nicht konfiguriert“. Die drei Zeilen für die explizite nil-Bindung erhöhen
die Größe von `modules/classday/compose` von 212 auf 213 Zeilen und die
Composition-Summe von 12.384 auf 12.385 Zeilen. Die eine zusätzliche gezählte
Zeile ist ausschließlich die Konstruktion des optionalen Ports; Fachlogik
bleibt außerhalb der Composition.

Nach dieser Entscheidung darf eine weitere Erhöhung des Budgets nicht auf sie
gestützt werden. Eine geänderte Bindung oder ein weiterer Konstruktor braucht
eine neue Entscheidung.
