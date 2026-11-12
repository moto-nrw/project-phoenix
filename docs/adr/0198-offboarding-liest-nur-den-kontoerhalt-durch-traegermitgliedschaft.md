---
status: proposed
---

# Offboarding liest nur den Kontoerhalt durch Trägermitgliedschaft

Für [#5974](https://github.com/moto-ogs/project-phoenix/issues/5974) bleibt eine
Trägermitgliedschaft mit Status `pending` oder `active` beim Schul-Offboarding
ein Grund, das Konto aktiv zu halten. Der Zustand des Trägers ändert daran
nichts, analog `auth.account_is_unused`. Die Anmeldung prüft weiterhin die
aktive Mitgliedschaft und den aktiven, nicht gelöschten Träger.

Organisation & Tenancy bietet dafür die einzelne Capability `AccountRetention`
an. Identity & Access bindet sie über seinen bestehenden Port
`TraegerMemberships`. Die breiten Fassaden `Query` und `Capability` wachsen
nicht. Beide Entzugspfade behalten ihre bestehende Transaktion und Kontosperre;
das Team-Portal bindet die Deaktivierungsentscheidung in die Vorschau-Revision.

`phoenix_tenant` darf `platform.organization_accounts` weiterhin nicht lesen.
Migration 1.15.904 bietet stattdessen eine `SECURITY DEFINER`-Funktion mit
festem Suchpfad an, die nur ein Boolean und nur für ein an der aktuellen
Schule aktiv zugeordnetes Konto liefert. Der Operator liest mit seinen
bestehenden Tabellenrechten. Es entstehen weder ein Scope-Wechsel noch
Cross-Owner-Schreibzugriffe. Das folgt ADR 0066 Punkt 6 und ADR 0067;
die Funktion ist eine Lese-Capability, kein Datenbank-Workflow.

Abgelehnt sind direkte Tabellenrechte für Schulen und ein Admin-Read außerhalb
der Offboarding-Transaktion: Ersteres legt Mitgliedschaften offen, Letzteres
verliert die Konsistenz unter der Kontosperre. Die kleine Capability vermeidet
außerdem die Erweiterung der breiten Organisations-Fassade.

Der zusätzliche Konstruktor benötigt 13 Zeilen Composition (343 → 356,
Gesamtsumme 13.232 → 13.245) und der transaktionsgebundene Owner-Read eine Delegation (13 → 14, Gesamtsumme 1.405 → 1.406). Diese eng
begrenzte Erhöhung betrifft ausschließlich die neue Capability dieses Issues. Sie erweitert keine Policy-Regel und kein Ausnahmeregister.
Die neue Methode im semantischen Delegationsinventar ist der
transaktionsgebundene Owner-Read. Die Root-Bindung bettet die Capability ein
und benötigt keine zusätzliche Weiterleitungsmethode.
