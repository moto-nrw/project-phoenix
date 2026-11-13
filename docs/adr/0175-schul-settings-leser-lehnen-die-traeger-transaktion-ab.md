---
status: proposed
---

# Schul-Settings-Leser lehnen die Träger-Transaktion ab

Für [#6221](https://github.com/moto-ogs/project-phoenix/issues/6221) erweitert
Settings Platform seinen bestehenden Runtime-Port um `IsTraegerTransaction`.
Nur die Composition bindet ihn an Tenant Runtime; die Application importiert
weder Tenant Runtime noch PostgreSQL. Der zentrale Schul-Leser prüft den
Transaktionstyp vor Snapshot, Request-Cache und Registry-Standard, ebenso die
expliziten Schul- und Batch-Leser. `HasTenantOverride` darf im falschen Kontext
nicht erfolgreich `false` liefern. Träger-Werte behalten ihre eigene Capability.

Ein Laufzeit-Guard prüft den tatsächlichen Kontext auch hinter Port-Bindungen
und bei Cache-Treffern. Ein statisches Aufrufregister müsste diese Bindungen
nachbilden und neue Leser laufend nachtragen. Die fünf zusätzlichen Zeilen der
Runtime-Abfrage sind die kleinere robuste Grenze. Der Composition-Etat von
`modules/settings/compose` steigt dafür genau von 315 auf 320 LOC und die Summe
von 13125 auf 13130; kein anderer Etat, Pass-through, Policy-Eintrag oder
Composition-Inventarschlüssel wächst. Die Zeilen bleiben im Ratchet erfasst.

[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
bleibt bindend: Ein Träger-Pfad liest Schulwerte in einem getrennten Schulzugriff
oder einem expliziten Admin-Batch, nicht in der Träger-Transaktion. Beim Ablösen
mit `ContextWithoutTransaction` entfernt Tenant Runtime auch die alte
Träger-Markierung. Das eröffnet keine Berechtigung; die aufrufende Capability
bestimmt weiterhin die zulässigen Schulen. RLS und ihre Rollen bleiben gleich.

Der Regressionstest setzt abweichende Schulwerte, öffnet eine echte
Träger-Transaktion und prüft Reader, Snapshots und Cache-Treffer. Ohne Guard
scheitern die Fehlerassertionen. Getrennte Zugriffe liefern 40 und 90 Tage,
echte fehlende Overrides den Registry-Standard; eine Schul-Transaktion darf
keine fremde Schule lesen. Der bestehende Mensa-Test aus #6169 schützt den
abgelösten Admin-Batch und den getrennten Schulzugriff.
