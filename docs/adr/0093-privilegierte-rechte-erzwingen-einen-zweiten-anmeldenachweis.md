---
status: accepted
---

# Privilegierte Rechte erzwingen einen zweiten Anmeldenachweis

Entscheidung für #3841, Slice 3841-f, Teil von #3846, am 23.10.2026.
Die Nutzerentscheidung ist Option B: Verwaltungsrechte sowie Gesundheits- und
Personalfinanzrechte verlangen MFA unabhängig von der Schuleinstellung.
Ergänzung vom 24.10.2026: Das gilt auch für `staff:stammdaten`,
`staff:documents`, `time_tracking:manage` und `vacation:approve`.
Grundlage ist die [Zugriffsbewertung für Gesundheits- und Personaldaten](../security/zugriff-gesundheits-und-personaldaten.md).

## Entscheidung

1. Identity & Access bewertet effektive Schulrechte, keine Rollennamen.
   Eine benannte Mindestmenge und ein vollständiger Registry-Klassifikationstest
   halten neue administrative Rechte sichtbar. Wildcards folgen der bestehenden
   Rechte-Semantik; neue Gesundheitsrechte fallen direkt unter die Untergrenze.
   `users:read` bleibt ausdrücklich außerhalb der Untergrenze, da sonst praktisch
   alle Betreuungskräfte erfasst würden. Gesundheitsangaben der Kinder
   (`health_info`) bleiben über dieses Recht sichtbar, ohne eigene Zugriffsstufe.
   Das ist keine Aussage, dass diese Angaben unbedenklich sind. Die Bewertung
   ihrer übrigen Zugriffsschutzmaßnahmen bleibt bestehen.
2. Kein `force_off` darf diese Untergrenze umgehen, auch nicht durch Operatoren.
   Bestehende Ausnahmen werden zur Laufzeit unwirksam. Die historischen Werte
   und Auditnachweise bleiben erhalten. Wiederherstellung repariert den zweiten
   Anmeldeweg, statt eine Ausnahme vom Schutz zu öffnen.
3. Eine persistierte Sitzungsmarkierung belegt den abgeschlossenen zweiten
   Nachweis. Neue Passwort-Sitzungen und Bestand haben diese Markierung nicht.
   Code-/Passkey-Austausch und bestätigte gemerkte Geräte setzen sie; Rotation
   und zulässige Antwortwiederherstellung erhalten sie. MFA-Zurücksetzung entzieht
   sie. Mint und Refresh prüfen aktuelle Rechte und Policy unter den vorhandenen
   Sperren erneut. Elternsitzungen sind ein eigener Bereich ohne Personalrechte.
4. Der Schulwechsel selbst ist keine zweite Authentisierung. Er erhält den
   vorhandenen Nachweis nur aus der vorgelegten, lebenden OGS-Sitzungsfamilie
   desselben Kontos, auch beim Wechsel in einen anderen Träger. Nicht die Schule
   oder der Träger, sondern diese Sitzung bindet den Nachweis. Identity & Access
   liest ihn unter der Kontosperre erneut. MFA-Zurücksetzung, Widerruf, Ablauf
   und fremde Portalbereiche dürfen keinen
   Nachweis liefern. Die Rechte und MFA-Regel der Zielschule gelten unverändert.
   Ohne gültigen Nachweis verlangt eine MFA-pflichtige Zielschule eine direkte
   Anmeldung; die API lehnt den Wechsel mit 401 und
   `identity.mfa_session_required` ab. Das gilt auch nach Ablauf der Schonfrist
   einer durch Schulwechsel stillgelegten Familie. Ein Login-Cache muss den
   vorgelegten Eintrag nach erfolgreichem Wechsel verwerfen. Er darf die
   zurückgegebene, lebende Zielsitzung übernehmen. Fehlt eine solche Sitzung,
   durchläuft die nächste Anmeldung unverändert den regulären MFA-Weg. Die bisherigen
   Operator- und Eltern-Anmeldeverfahren bleiben unverändert.

## Folgen

- Bestehende privilegierte Sitzungen verlangen beim nächsten Refresh eine neue
  Anmeldung. Bereits ausgegebene Access-Token laufen regulär ab.
- Berechtigte Konten ohne zweiten Faktor gelangen in die bestehende Einrichtung.
  Das gilt auch für Konten mit ausschließlich einem der vier ergänzten Rechte.
  Ohne erreichbares Postfach oder gültigen Passkey muss Support zunächst diesen
  Anmeldeweg wiederherstellen. Es gibt keine Passwort-Notfallausnahme.
- Der dev-only Seed-Adapter bestätigt dieselben E-Mail-Codes über Mailpit.
  Die API erhält keinen Entwicklungs-Bypass.
- [Runbook](../operations/mfa-pflicht-3841.md) trennt Implementierung und offene
  produktive Nachweise. Der AVV/TOM-Text ist ein Vorschlag, kein Betriebsnachweis.
