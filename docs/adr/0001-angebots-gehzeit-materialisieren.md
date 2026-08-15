# Angebots-Gehzeiten werden auf Kind-Gehzeiten ausgerollt, nicht zur Lesezeit aufgelöst

Betreuungsangebote (`enrollment.care_offerings`) tragen optionale Gehzeiten pro
Wochentag (#2290). Damit diese Zeiten "überall" erscheinen (Klassenliste,
Kinderdetail, Kindersuche), haben wir uns entschieden, sie beim Speichern des
Angebots und bei der Genehmigung von Anmeldungen als konkrete Zeilen in
`schedule.student_pickup_schedules` zu **materialisieren** (mit Herkunftsfeld
`source` + Angebots-Referenz), statt sie zur Lesezeit als Fallback aus dem
Angebot aufzulösen.

## Considered Options

- **Lesezeit-Fallback**: kein Schreiben, jeder Leser der geplanten Abholzeit
  fällt bei fehlendem `student_pickup_schedules`-Eintrag auf die Gehzeit des am
  Wochentag gebuchten Angebots zurück. Verworfen: die Auflösungskette
  Kind → genehmigte Anmeldung → `request_child_offerings` → Angebot liegt im
  Enrollment-Domain, während die geplante Abholzeit im Schedule-Domain
  aufgelöst wird (`care_day_resolver.go` liest bewusst keine Angebote). Der
  Fallback hätte diesen Domain-Übertritt in jeden Lesepfad getragen — eine
  "Referenz auf eine Referenz", die bei jedem neuen Konsumenten erneut
  verdrahtet werden muss.
- **Materialisierung** (gewählt): einmaliger Domain-Übertritt an zwei
  Schreibstellen (Rollout, Genehmigung); alle bestehenden Lesepfade
  funktionieren unverändert.

## Consequences

- Die Materialisierung muss aktiv konsistent gehalten werden: Gehzeit-Änderung
  am Angebot → erneuter Rollout (Bestätigungsdialog, manuell abweichende Kinder
  gelistet und einzeln ausnehmbar); Genehmigung/Rollover → automatisches
  Schreiben; Buchungsverlust → Entfernen der Angebot-stämmigen Einträge.
  Manuell gepflegte Einträge (Herkunft `staff`) sind vor automatischen Pfaden
  geschützt.
- `schedule.student_pickup_schedules` braucht das Herkunftsfeld dauerhaft; ein
  späterer Wechsel zur Lesezeit-Variante hieße Daten-Rückbau über alle Tenants.
