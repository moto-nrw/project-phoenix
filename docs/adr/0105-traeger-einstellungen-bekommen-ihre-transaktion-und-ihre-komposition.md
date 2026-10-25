---
status: accepted
---

# Träger-Einstellungen bekommen ihre Transaktion und ihre Komposition

Entscheidung vom 25.10.2026 zu [#4652](https://github.com/moto-nrw/project-phoenix/issues/4652),
erster Bau-Schnitt von
[ADR 0086](0086-traeger-werte-liegen-zwischen-standard-und-schule.md).
ADR 0086 hat die Träger-Ebene im Settings-System beschlossen. Diese ADR hält
fest, was ihr erster Code in den Registern ändert, damit kein Register
stillschweigend wächst (ADR 0057 Punkt 5, ADR 0070), und welche Schlüssel
v1 auf die Träger-Ebene legt (ADR 0086 Abschnitt 4: „Die Liste je Schlüssel
legt das Bau-Ticket fest“).

## Entscheidung

1. **Träger-Transaktion.** `tenant.WithinTraeger` öffnet die
   Träger-Transaktion aus ADR 0086 Abschnitt 5: Rolle `phoenix_tenant`,
   `app.current_org_id` aus der Träger-Sitzung, keine Schule. Der
   Postgres-Adapter der Transaction Runtime öffnet sie über dieselbe Stelle
   `withinGlobal`, die schon die schulfreien Einheiten öffnet; der
   Demo-Runtime fehlt sie. Eine verschachtelte Träger-Transaktion schließt sich
   nur einer Träger-Transaktion desselben Trägers an.
2. **Register der Öffner.** `backend/test/transaction_boundaries.json`
   bekommt den Abschnitt `traeger`. `TestTransactionBoundaryRatchet` erkennt
   `WithinTraeger` und verlangt für jede Stelle einen Eintrag mit Grund. Der
   Abschnitt ist aufgezählt, nicht nur schrumpfend: ADR 0086 macht die
   Träger-Transaktion zum Muster für jede Tabelle des Trägers. Admin- und
   Command-Register bleiben unverändert.
3. **Budgets.** `moduleComposeBudgets` steigt um genau 15 Zeilen:
   `modules/settings/compose` von 308 auf 322 (Speicher, Runtime-Bindung und
   Dienst der Träger-Ebene) und `modules/transactionruntime/compose` von 133
   auf 134 (Bindung der Träger-Transaktion). `moduleComposeBudgetTotal`
   steigt von 12411 auf 12426. Kein anderer Eintrag wächst.
4. **Lesen der Schule.** Die Settings Platform liest `platform.schools` nie.
   In der Tenant-Transaktion einer Schule wählt die lesende Policy der Tabelle
   den Träger der Schule; die Abfrage verlangt zusätzlich, dass
   `app.current_tenant_id` diese Schule ist, und liest sonst nichts. Der
   Batch-Lesepfad über mehrere Schulen fragt Organisation & Tenancy
   (`ListSchoolsByID`) nach dem Träger jeder Schule, bevor er seine
   Admin-Transaktion öffnet.
5. **Benachrichtigung.** `tenant_settings_changed` geht nach dem Commit der
   eigenen Träger-Transaktion an jede Schule des Trägers. Es entsteht kein
   neuer Rückruf nach dem Commit; `durable_side_effect_notices.json` bleibt
   unverändert.
6. **Aufbewahrung.** Jede Änderung eines Trägers löscht in derselben
   Transaktion seine Einträge, die älter als die Frist des Schul-Audits sind
   (drei Jahre, #4044). Mit dem Träger gehen alle Einträge.
7. **Einordnung v1.** `beide` bekommen die Rechtstexte der Anmeldung (vier
   Schalter und vier Texte, ohne Anzeigeart und PDF der AGB, die an der Datei
   der Schule hängen) und die Funktionen des Elternportals ohne Sperre (neun
   Schlüssel). Alles andere bleibt `schule`. Lohnarten, Einheiten und
   DATEV-Nummern folgen mit dem Personal-Schnitt: Sie leben auf einer eigenen
   Seite der Schule, die die Herkunft eines Werts noch nicht zeigt. `traeger`
   bekommt in v1 keinen Schlüssel, weil die Werte (Branding, Vertragsweg,
   Kündigungsfrist) noch keinen Verbraucher haben.

## Folgen

- Weitere Tabellen des Trägers öffnen die Träger-Transaktion mit je einem
  Eintrag im Abschnitt `traeger`. Ein wachsendes Compose-Budget braucht
  wieder eine eigene ADR.
- Die Liste der Schulen, die bei einer Einstellung abweichen (ADR 0086
  Abschnitt 6), liest über Schulgrenzen und gehört deshalb in die Projektion
  `traeger-view`. Sie folgt mit eigener ADR.
- Ein Träger ohne neue Änderung behält ältere Einträge bis zur nächsten
  Änderung oder bis zu seiner Löschung.
