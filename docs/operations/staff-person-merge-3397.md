# Bestehende Personal-Doppelsätze zusammenführen (#3397)

Das Operator-Portal bietet unter **Personen** nach der Schulauswahl den Abschnitt
**Doppelte Personaleinträge prüfen**. Es führt nur Personen zusammen, keine
Beschäftigungen. ADR 0021 bleibt das Modell: eine Person je Mensch und Schule,
mehrere Beschäftigungen möglich.

1. Mit der Schule jeden vorgeschlagenen Fall klären. Gleiche Namen beweisen
   keine gemeinsame Identität. Erstellungszeitpunkte, Beschäftigungszeiträume,
   Konto und gegebenenfalls Geburtsdatum auf beiden Seiten vergleichen.
2. Jeden bestätigten Fall einzeln auswählen. Unklare Fälle nicht auswählen.
3. Die Abstimmung mit der ausgewählten Schule bestätigen.
4. Die geprüften Fälle gemeinsam ausführen.

Der Personensatz mit Konto bleibt erhalten, unabhängig von seiner zeitlichen
Reihenfolge. Die Beschäftigung des anderen Personensatzes wird diesem zugeordnet.
Der andere Personensatz wird weich gelöscht; historische Fremdschlüssel auf ihn
bleiben bestehen. Ein fehlendes Geburtsdatum des verbleibenden Satzes wird aus
dem anderen übernommen. Alle Beschäftigungs-IDs, Zeitkonten, Urlaubskonten,
Monatsabschlüsse und bestehenden Auditzeilen bleiben unverändert. Die alten
Beschäftigungen bleiben über die Personenverknüpfung erreichbar. Es entsteht
kein gemeinsamer Stunden- oder Urlaubssaldo.

Die Vorschläge verlangen genau zwei lebende Personen mit gleichem Vor- und
Nachnamen (ohne Beachtung der Großschreibung) an derselben Schule, genau ein
Konto und jeweils genau eine Beschäftigung, davon eine frühere und eine aktive.
Ein Kinderprofil schließt die Person aus, auch wenn es zusätzlich eine
Beschäftigung gibt. Mehrdeutige Gruppen werden nicht angeboten. Widersprüchliche
Geburtsdaten, ein Transponder am abzugebenden Satz oder bereits anonymisierte
Beschäftigungen werden ebenfalls nicht angeboten. Solche Fälle benötigen eine
eigene Prüfung; fehlende Vorschläge sind kein Nachweis, dass es keine Dubletten
gibt.

Die Ausführung prüft die ausgewählten Vorschauen erneut unter Zeilensperren.
Eine veränderte Auswahl bricht den gesamten Stapel ab. Auch ein Fehler beim
Schreiben des Operator-Audits rollt den gesamten Stapel zurück. Das Audit hält
Operator, Schule, beide Personen- und Beschäftigungs-IDs sowie die bestätigte
Schulabstimmung fest. Nach einem Fehler Vorschläge neu laden und die Personenliste
prüfen, bevor erneut ausgewählt wird. Bei einem Verbindungsabbruch kann der
Stapel bereits abgeschlossen sein.

Es gibt keine automatische Datenmigration und keinen Lauf beim Deployment.
Dieses Werkzeug ersetzt weder die Einzelfallprüfung noch die Abstimmung mit der
Schule. Eine spätere Zusammenführung der Beschäftigungen wäre eine gesonderte
fachliche und technische Entscheidung.
