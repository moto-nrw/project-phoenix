---
status: accepted
---

# Ferienzeitraum liegt beim Träger in der Träger-Transaktion

Entscheidung vom 25.10.2026 zu [#4659](https://github.com/moto-nrw/project-phoenix/issues/4659),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 22 bis 25. Sie ist der erste Schnitt von
[ADR 0086](0086-traeger-werte-liegen-zwischen-standard-und-schule.md)
Abschnitt 5: ADR 0086 hat die Träger-Transaktion beschlossen und gesagt, dass
auch der Ferienzeitraum sie nutzt. Diese ADR hält fest, wo sie im Code
entsteht und was sie in den Registern ändert, damit kein Register
stillschweigend wächst (ADR 0057 Punkt 5, ADR 0070). Sie führt keinen neuen
Owner ein.

## Entscheidung

1. **Owner:** Der Ferienzeitraum mit Wochen, ausrichtender OGS und
   Zeitmodellen gehört Enrollment, wie die Spec es unter „Enrollment und Care
   Plan“ vorsieht und
   [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
   Abschnitt 2 für Objekte auf Träger-Ebene festlegt. Die Tabellen
   `enrollment.holiday_periods`, `enrollment.holiday_period_weeks` und
   `enrollment.holiday_week_time_models` tragen `organization_id` und kein
   `tenant_id`. Die bestehende Online-Anmeldung je Schule bleibt unverändert;
   die öffentliche Ferienanmeldung (#4661) baut auf diesen Zeiträumen auf.
2. **Träger-Transaktion:** `tenant.WithinTraeger` öffnet sie. Der
   PostgreSQL-Adapter von Transaction Runtime setzt die Rolle `phoenix_tenant`
   und `app.current_org_id`, aber keine Schule. Die Tabellen haben eine
   Policy `organization_id = app.current_org_id` mit `FORCE ROW LEVEL
   SECURITY`. Eine Schul-Transaktion setzt `app.current_org_id` nie und sieht
   deshalb keine Zeile; eine Träger-Transaktion sieht keine Schultabelle, weil
   `app.current_tenant_id` fehlt. Eine Träger-Transaktion verschachtelt sich
   weder in eine Schul- noch in eine Admin-Transaktion und umgekehrt. Die
   Demo-Laufzeit bekommt keine.
3. **Öffnen:** Nur `common.TraegerTxMiddleware` öffnet sie für eine Route des
   Träger-Portals, mit dem Träger der Sitzung. `ProtectedTraegerRoutes`
   reicht sie wie `ProtectedTenantRoutes` die Schul-Transaktion an die
   Routen weiter. Kein Owner-Befehl öffnet sie selbst. Das Postgres-Paket
   von Enrollment prüft vor jeder Anweisung, dass es in der
   Träger-Transaktion genau dieses Trägers läuft, und nennt den Träger in
   jedem Filter noch einmal.
4. **Transaktionsregister:** Der Scanner von
   `backend/test/transaction_boundaries_ratchet_test.go` erkennt
   `WithinTraeger` als Beginn einer Einheit. Das Register erhält genau vier
   Stellen: den Einstieg in `tenant/traeger.go`, die Bindung in
   `modules/transactionruntime/compose`, den Adapter und
   `TraegerTxMiddleware`. `backend/test/traeger_transaction.go` lässt nur
   diese vier Schlüssel mit ihren festen Gründen am Schrumpf-Vergleich
   vorbei. Jede weitere Stelle braucht eine eigene Entscheidung.
5. **Ausrichtende OGS:** Ein Trigger lässt als ausrichtende OGS nur eine nicht
   gelöschte Schule des Trägers des Zeitraums zu. Die Datenbank entscheidet,
   Enrollment benennt die Ablehnung. Die Spalte ist kein Fremdschlüssel auf
   `platform.schools`: Die Zeilen gehören dem Träger und gehen mit ihm, die
   Schullöschung (ADR 0092) plant nur Zeilen der Schule. Wird eine
   ausrichtende Schule später gelöscht, bleibt die bloße ID stehen, bis das
   Träger-Büro die Woche ändert.
6. **Preise:** Jedes Zeitmodell trägt seinen Preis als Ganzzahl in Cent. Im
   Code steht kein Preis. Den Grundpreis daraus berechnet die
   Elternabrechnung (#4664) und hält ihn bei der Bestätigung fest
   ([ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)).
7. **Schreiben über mehrere Owner:** Anlegen, Ändern und Veröffentlichen
   schreiben nur Enrollment. Recht und Träger kommen aus der Sitzung, die
   Zugehörigkeit der Schule prüft die Datenbank. Es gibt deshalb keinen
   Workflow. Sobald eine Veröffentlichung in Schulen schreibt, läuft das
   nach ADR 0084 Abschnitt 2 als Workflow mit einem Tenant-Schritt je Schule.

## Folgen

- Weitere Tabellen des Trägers (Träger-Werte nach ADR 0086, Tarif, Vorlage)
  nutzen dieselbe Middleware und dieselbe Policy. Sie brauchen keine neue
  Stelle im Transaktionsregister, wohl aber ihre eigene Policy.
- Die Lese-Policy für die Schule nach ADR 0086 Abschnitt 5 (die ausrichtende
  OGS sieht ihre Wochen) kommt mit der Teilnehmerliste (#4665).
- Der Ferienzeitraum enthält keine personenbezogenen Daten. Ein Zugriff
  darauf steht deshalb nicht im Zugriffsprotokoll (ADR 0084 Abschnitt 7).
