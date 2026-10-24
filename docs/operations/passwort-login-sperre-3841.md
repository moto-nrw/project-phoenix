# Kontobezogener Anmeldeschutz (Slice 3841-c)

## Stand und Grenzen

**Im Code umgesetzt:** Passwortversuche zählen je Konto, unabhängig von IP,
Schule und Portal. OGS-, Schul- und Elternportal teilen den Kontozähler.
Operatoren haben einen eigenen Zähler. PostgreSQL-Zeilensperren serialisieren
parallele Versuche auch bei mehreren Backend-Prozessen. Die vorhandene
IP-/Routenbegrenzung bleibt bestehen.

**Produktiv nachzuweisen, offen:** Migration, Rollout aller Backend-Prozesse,
funktionierende Anmeldung in allen vier Portalen und das Verhalten bei
verteilten Versuchen. Lokale Tests sind kein produktiver Nachweis. Dieser Slice
belegt #3841 AK5 im Code und schließt die bekannte MFA-Reset-Race aus AK3;
andere Akzeptanzkriterien bleiben bei ihren jeweiligen Slices.

## Grenzwerte und Konfiguration

Nach **fünf falschen Passwörtern** pausiert die Passwortanmeldung für
**15 Minuten**. Ein korrektes Passwort setzt den Passwortzähler zurück, sofern
die Sperre nicht mehr gilt. Der anschließende MFA-Schritt bleibt unabhängig:
Ein korrektes Passwort setzt niemals MFA-Code-Versuche zurück.

Ein Versuch während der Sperre verlängert sie nicht, auch nicht mit dem
richtigen Passwort. Nach Ablauf beginnt ein neuer Versuchshaushalt. Ein
Passwortwechsel oder eine Passwort-Zurücksetzung hebt die laufende Sperre
nicht auf. Bestehende Sitzungen und Passkeys bleiben davon unberührt.

Die Werte stammen zentral aus den Registry-Defaults
`security.account_lockout_threshold` und
`security.account_lockout_duration_minutes`. Änderungen dieser Defaults
erfordern einen geprüften Code-Rollout. Es gibt keinen Env-Fallback und kein
zusätzliches Plattform-Einstellungsmodell. Schul-Overrides dieser bestehenden
Einstellungen gelten weiterhin **nur für MFA-Codes**. Ein Schulwechsel oder
anderes Portal darf eine plattformweite Identitätssperre nicht abschwächen.

Die Wahl begrenzt verteiltes Raten, ohne eine dauerhafte Support-Sperre zu
erzeugen. Sie erlaubt einem Angreifer dennoch eine zeitweise Zugangsstörung.
Deshalb verlängern gesperrte Versuche die Frist nicht. Fehlermeldung und
HTTP-Status unterscheiden unbekannte Konten, falsche und gesperrte Passwörter
nicht. Auch für unbekannte Konten und Konten ohne Passwort erfolgt eine
Argon2id-Prüfung gegen einen zur Laufzeit erzeugten Vergleichswert. Gleiche
Laufzeiten sind damit angenähert, nicht garantiert: Datenbankzugriffe und
Sperrkonkurrenz unterscheiden sich weiterhin.

## Hilfe und Entsperren

1. Eingaben und richtige Portaladresse prüfen. Nicht wiederholt raten.
2. Bei mehreren Fehlversuchen 15 Minuten warten, dann erneut anmelden.
3. Falls bereits vorhanden, den eigenen Passkey nutzen. Für eine vergessene
   Eingabe den bestehenden Passwort-Reset nutzen; die Wartezeit bleibt bestehen.
4. Bei anhaltenden Problemen prüft berechtigter Support Kontostatus, Einladung
   und Schulzuordnung über die vorhandene Kontoverwaltung. Es gibt in diesem
   Slice keine neue manuelle Entsperrfunktion und keinen Passwort-Bypass.

## MFA und Wiederherstellung

MFA-Code-Versuche haben bereits einen eigenen atomaren Kontozähler: Standard
fünf Versuche, 15 Minuten Sperre, mit den bestehenden Schul-Overrides. Operatoren
verwenden fünf Versuche und 15 Minuten. Die Code-Anforderung ist auf drei
Codes pro 15 Minuten und Identität begrenzt; die Persistierung prüft diese
Grenze nochmals unter der Identitätssperre. Ein neuer Code umgeht keinen
MFA-Fehlversuchszähler.

Passwort-Reset-Anforderungen nutzen bereits einen persistenten atomaren Zähler
je normalisierter Adresse: drei pro Stunde. Er gilt portalübergreifend. Der
bestehende Vertrag `429` mit `Retry-After` bleibt erhalten. Unbekannte oder
für das Portal ungeeignete Konten erzeugen keine Mail und keinen Zählereintrag.
Damit kann die bestehende Reset-Antwort nach wiederholten Anforderungen
weiterhin von unbekannten Konten abweichen; dieser Slice ändert den bestehenden
Countdown-Vertrag nicht. Operatoren verwenden ihren bestehenden
administrativen Wiederherstellungsweg, keinen neuen öffentlichen Reset.

Ein MFA-Reset erhöht atomar die Reset-Generation des Kontos und entzieht
bestehenden Sitzungen den MFA-Nachweis. Challenges und erfolgreiche
Code-Prüfungen tragen ihre Generation. Tenant- und Schul-Mint vergleichen
sie unter derselben Kontosperre wie der Reset. Eine vor dem Reset geprüfte
Challenge darf danach weder eine Sitzung noch einen Geräte-Cookie erzeugen.
Auch das Merken eines Geräts erfasst seine Generation vor dem Mint und prüft
sie bei der Ausgabe unter der Kontosperre. Ein Reset nach dem Mint kann
dadurch keinen neuen gemerkten Zugang hinterlassen.
Der Vergleich gilt auch für aktuell nicht MFA-pflichtige Schulen, damit ein
späterer Schulwechsel keinen alten Nachweis weitertragen kann. Bereits
ausgestellte Access-Token laufen nach dem bestehenden Vertrag aus.

## Überwachung ohne Kontodaten

Über einen freigegebenen administrativen, nur lesenden Datenbankzugang:

```sql
SELECT count(*) AS gesperrte_passwortkonten
FROM auth.accounts
WHERE personal_data_erased_at IS NULL AND password_locked_until > CURRENT_TIMESTAMP;

SELECT count(*) AS gesperrte_operatoren
FROM platform.operators
WHERE password_locked_until > CURRENT_TIMESTAMP;
```

Nur aggregierte Anzahlen im Betriebsnachweis ablegen. Die bestehenden
fehlgeschlagenen Auth-Ereignisse und HTTP-Fehlerraten helfen bei der Einordnung;
sie enthalten keinen neuen Sperrcode für einen unauthentisierten Absender.
Keine Passwörter, Codes, vollständigen Adressen oder Kontolisten kopieren.

## Rollout und lokale Nachweise

Migration **1.15.499** ergänzt Zähler, Fristen und Reset-Generationen an
bestehenden Identity-&-Access-Tabellen. Kein neuer Datentopf, keine zusätzliche
Aufbewahrungsfrist. Die neuen Sicherheitsfelder sind im Laufzeitkatalog
aus Kundenexporten ausgeschlossen; die bestehende globale Kontolöschung
bleibt zuständig. Backend und Migration zusammen ausrollen; alte Prozesse
zählen Passwortversuche nicht. Ein gemischter Rollout ist kein Wirksamkeitsnachweis.
Ein Rollback entfernt den Schutz und benötigt eine gesonderte Entscheidung.

Die Regressionstests in `backend/api/password_login_protection_test.go` und
`backend/api/mfa_reset_exchange_test.go` prüfen Sperre, Ablauf, Zurücksetzen,
Parallelität, Portalgrenzen und den zwischen Prüfung und Mint committeten Reset.
`backend/api/mfa_challenge_rate_race_test.go` prüft parallele Code-Anforderungen.
Die Tests erzeugen Zugangsdaten zur Laufzeit und verwenden nur lokale Testdaten.

## Vorschlag AVV/TOM, kein Betriebsnachweis

„Passwortanmeldungen werden zusätzlich zur Anfragenbegrenzung kontobezogen
geschützt. Wiederholte Fehlversuche führen zu einer zeitlich begrenzten Sperre,
die über IP-Adressen und Portalzugänge hinweg gilt. Zweite Anmeldenachweise
und Wiederherstellung besitzen eigene Fehlversuchsgrenzen. Ein Zurücksetzen
des zweiten Nachweises entzieht auch bestehenden Sitzungen diesen Nachweis;
nebenläufige alte Prüfungen dürfen ihn nicht wiederherstellen.“

Vor Übernahme Rollout, Grenzwerte, Wiederherstellung und Überwachung am
produktiven System durch befugte Menschen belegen.
