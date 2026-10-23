# RLS-Abdeckung: Slice 3842-a

## Im Code umgesetzt und lokal geprüft

`TestEveryTenantTableHasRLS` prüft nach allen Migrationen jede reguläre oder
partitionierte Tabelle mit einer nicht gelöschten Spalte `tenant_id` in allen
Nicht-Systemschemata. Partitionsblätter sind ausgenommen. Der Test verlangt
aktiviertes und erzwungenes RLS, eine Policy und eine für `phoenix_tenant`
geltende Policy. Er beweist nicht die Semantik beliebiger Policy-Ausdrücke;
ein Texttreffer auf `tenant_id` und `current_setting` wäre dafür unzureichend.

Die Ausnahmen in `rls_coverage_test.go` dürfen nur schrumpfen. Sie betreffen
administrative Tabellen ohne direkte Tabellen- oder Spaltenrechte für
`phoenix_tenant`: zwei Migrationsbackups, die Push-Zustellungsmarker mit
tenant-gefilterten SECURITY-DEFINER-Funktionen und den Demo-Steuerzustand mit
eigener `phoenix_demo`-Policy. Neue Rechte, fehlende Tabellen, leere Begründungen
oder inzwischen erfüllte RLS-Abdeckung lassen den Test fehlschlagen.

Migration `1.15.475` ergänzt RLS für `auth.account_tenants`,
`auth.mfa_email_challenges`, `auth.mfa_trusted_devices`, `auth.passkey_sessions`
und `auth.parent_declaration_permission_grants`. Vorhandene Datensätze bleiben
unverändert. Die bisher berechtigte Voranmelderolle `phoenix_auth` behält ihren
Zugriff durch eigene Policies; `phoenix_tenant` erbt diese Rolle nicht.
Admin-/Worker-Zugriffe bleiben auf `phoenix_admin` mit BYPASSRLS.
Nur die explizit markierten accountweiten Passkey- und Kalender-Token-Kommandos
öffnen ohne bestehende Transaktion die administrative Einheit, auch wenn der
Kontext bereits einen Schulhinweis trägt. Laufende Transaktionen werden nicht
ersetzt; Sitzungszugriffe und andere Schulzugriffe behalten ihre bisherige
Transaktionsauswahl und ihre RLS-Grenze.
Die bestehende Träger-Kontoverwaltung nutzt wie ihre Rollenliste eine separate
administrative Transaktion. Ihre Lese- und Schreibprädikate begrenzen sie
weiterhin auf die Schulen des Trägers; eine Schultransaktion darf dafür nicht
auf weitere Schulen erweitert werden.
Für Offboarding gibt `auth.active_school_ids_for_account` nur aktive Schul-IDs
eines Kontos zurück, das an der aktuellen Schule aktiv zugeordnet ist.
Fremde Zuordnungszeilen und Kalender-Token bleiben unzugänglich. Die eng
begrenzte SECURITY-DEFINER-Funktion hat einen festen Suchpfad und keine
PUBLIC-Ausführungsrechte; Voranmelde- und Admin-Rollen behalten ihre bisherigen
globalen Abfragen. Sie verwendet dieselbe Transaktionssicht wie das Offboarding.
Die Rückmigration verweigert das Wiederöffnen der Lücken. Ein älteres App-Image
muss mit beibehaltenen Policies laufen; ein Datenbank-Restore braucht den
erneuten Sicherheitsabgleich vor regulärer Nutzung.

Die Regressionstests belegen mit synthetischen Daten zweier Schulen, dass
Fremdlesen, Fremdändern, Fremdlöschen und Umhängen in eine fremde Schule unter
der Request-Rolle nicht möglich sind. Eigene Zeilen bleiben sichtbar;
Voranmeldezugriffe funktionieren weiterhin. Ohne Schulkontext sieht die
Request-Rolle keine Zeilen.

## Produktiv nachzuweisen: offen

Dieser Slice enthält keinen Zugriff auf Produktion oder Staging und bestätigt
keinen dortigen Zustand. Nach dem regulären Deploy führt ein berechtigter
Mensch folgende Nachweise mit dem bestehenden beschränkten Diagnosezugang:

1. Ausgerollten Commit und erfolgreichen Lauf von Migration `1.15.475` belegen.
2. Im Datenbankkatalog für die fünf Tabellen `pg_class.relrowsecurity`,
   `relforcerowsecurity` und die zugehörigen `pg_policies` prüfen. Die
   Tenant-Policy muss `tenant_id` gegen `app.current_tenant_id` prüfen;
   die Voranmelde-Policy darf nur `phoenix_auth` gelten.
   Bei `auth.active_school_ids_for_account` festen Suchpfad, fehlendes
   PUBLIC-Ausführungsrecht und die lokale aktive Kontozuordnung als Bedingung
   für die Schulrolle bestätigen.
3. Die vier administrativen Ausnahmen gegen die Liste im Test abgleichen:
   keine effektiven Tabellen- oder Spaltenrechte für `phoenix_tenant`.
   Für den Demo-Steuerzustand RLS/FORCE und die Demo-Policy bestätigen.
4. Ergebnis, Zeitpunkt und Commit ohne Zugangsdaten oder Datensätze im internen
   Deploy-Nachweis festhalten. Abweichungen bleiben offen und werden behoben,
   nicht durch neue Ausnahmen verdeckt.

Weitere Kriterien von #3842, etwa TLS, CSP, Deploy-Rechte und Betriebsprotokolle,
sind nicht Teil dieses Slices. Tabellen ohne `tenant_id` sind nicht abgedeckt.
