# Operator-Zugriffe und Audit-Ausfälle (Slice 3842-b)

## Im Code umgesetzt

Operator-Anfragen auf Personen- und Kontolisten schreiben vor dem Lesen einen
Eintrag mit `action=view` nach `platform.operator_audit_log`. Das gilt für
schulbezogene, trägerbezogene und globale Kontolisten sowie schul- und
trägerbezogene Personenlisten. Kontodetails zu Betreuung, MFA und Schulzugängen
werden ebenfalls erfasst, einschließlich beider Schreibweisen der
Schulzugangsliste mit und ohne abschließenden Schrägstrich.

Ein Eintrag bezeichnet einen **Zugriffsversuch**, keinen bestätigten Empfang.
Er enthält Operator-ID, Zeitpunkt, Anfrage-IP, eine feste Ansichtskategorie und
gegebenenfalls Schul-, Träger- oder Konto-ID. Keine Ergebniszeilen, Namen,
E-Mail-Adressen, Suchbegriffe, Anfragekörper oder Zugangsschlüssel. Es entsteht
ein Eintrag pro Anfrage, auch bei leeren Listen; spätere Lesefehler können einen
bereits protokollierten Versuch hinterlassen. Die bestehenden Operator-Sitzungs-
und Berechtigungsprüfungen bleiben davor bestehen.

Folgende Änderungen geben Audit-Fehler an ihre bestehende administrative
Transaktion zurück und rollen mit ihr zurück:

- Geräte-Schlüsselrotation, Geräte-Löschung und Schultransfer eines Geräts.
- Soft-Delete von Schule, Träger und Person. Die Träger-Löschung war bereits
  transaktional protokolliert; sie verwendet nun denselben Fehlercode.
- Schulbezogener und globaler MFA-Override, auch das Entfernen eines Overrides,
  sowie das Zurücksetzen der MFA-Anmeldung durch einen Operator. Das neue
  schulbezogene Operator-Protokoll speichert keine Freitextbegründung.

Bei diesen Audit-Ausfällen und bei nicht protokollierbaren Lesezugriffen antwortet
die API mit `503`, Code `operator.audit_unavailable`. Der Operator erhält eine
Fehlermeldung; es gibt keinen Erfolg und keine Datenfreigabe. Das Betriebslog
meldet den Audit-Ausfall mit fester Aktionskategorie, ohne Anfrageinhalt.
Andere bisherige best-effort-Auditpfade werden nicht pauschal umgestellt.
Eine künftige endgültige Mandantenlöschung muss ihren Audit-Schritt ebenfalls
in ihre Löschtransaktion aufnehmen; dieser Slice implementiert sie nicht.

Das Schema erlaubt die neue Aktion ohne Migration. Eigentümer des Protokolls,
Datenbankrechte und Löschlauf bleiben unverändert. Die Aufbewahrung endet
weiterhin am Ende des Folgejahres, siehe
[Fristenliste](../reference/loeschfristen.md). Es wird keine zusätzliche Frist
und keine allgemeine gesetzliche 30-Tage-Frist eingeführt.

## Lokaler Nachweis

Die Tests verwenden ausschließlich synthetische Fixtures und isolierte
Testdatenbanken. Ein Trigger verweigert das Einfügen von Audit-Zeilen. Geprüft
werden Rollback von Schlüssel, Transfer, Löschungen und MFA-Zustand sowie die
Sperre der gemounteten Personen-/Kontenlisten. Port-Fehlerinjektion prüft auch
Audit-Ausfälle, die PostgreSQL nicht selbst in einen Fehlerzustand versetzen.

```sh
scripts/run-go-toolchain.sh go -C backend test -tags=testhooks ./api \
  -run 'TestOperatorAuditFailure|TestOperatorPersonAndAccountRoutesAuditOneAttemptPerRequest' -count=1
scripts/run-go-toolchain.sh go -C backend test -tags=testhooks \
  ./modules/organizationtenancy/internal/application \
  ./modules/organizationtenancy/inbound/operator
```

## Produktiv durch Menschen nachzuweisen

Dieser Code-Nachweis bestätigt keinen produktiven Stand. Nach dem Rollout muss
der Betrieb mit einem freigegebenen internen Testkonto prüfen, dass die neuen
`view`-Einträge ankommen und Audit-Ausfälle als Betriebsfehler erkannt werden.
Fehlerinjektion nur in einer isolierten Testumgebung durchführen. Keine echten
Schlüssel, Namen, Adressen oder Antwortinhalte als Nachweis übernehmen.

Zu bestätigen bleiben die produktiven Protokollrechte, Überwachung der
Audit-Ausfälle und der bereits vorhandene Löschlauf. Das Ergebnis wird vom
Orchestrator im Ergebnisbericht 3842-f zusammengeführt. Dieser Slice bearbeitet
dessen Datei nicht.
