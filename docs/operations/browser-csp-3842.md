# Browser-Schutz: Slice 3842-e

Stand: 23.10.2026. Teil von #3842, Akzeptanzkriterium 4, im Epic #3846.
Diese Bewertung betrifft die CSP und die unten genannten Anwendungspfade,
nicht sämtliche Browser-Angriffswege oder den produktiven Sicherheitszustand.

## Im Code umgesetzt und lokal geprüft

Der Abgleich mit `origin/development` (`d780efb392`) zeigt: `base-uri 'self'`
und `form-action 'self'` waren bereits vorhanden. `unsafe-eval` war schon
auf `NODE_ENV=development` begrenzt. Neu ist `object-src 'none'` in der
gemeinsamen CSP von `frontend/src/proxy.ts`.

- `object-src 'none'` sperrt eingebettete Objekte, auch von derselben Origin.
  Bisher erlaubte der Rückfall auf `default-src 'self'` solche Inhalte.
  Im Anwendungscode sind keine `<object>`- oder `<embed>`-Einbindungen gefunden.
- `base-uri 'self'` begrenzt die Basisadresse relativer URLs auf die eigene
  Origin; `form-action 'self'` begrenzt HTML-Formularziele auf dieselbe Origin.
  Beides bleibt unverändert. Diese Regeln begrenzen nicht beliebige
  JavaScript-Navigationen oder alle Datenabflüsse bei bereits ausgeführtem XSS.
- `frame-ancestors 'none'`, `X-Frame-Options: DENY`, `nosniff` und
  `Referrer-Policy: strict-origin-when-cross-origin` bleiben erhalten.
- Nur das öffentliche Tenant-Phasenformular erhält die Turnstile-Ausnahme für
  Skripte und Frames von `https://challenges.cloudflare.com`. Eltern-, Schul-,
  Operator-, Vorschau- und Statusseiten erhalten diese Ausnahme nicht.
  PostHog nutzt weiterhin den Same-Origin-Pfad `/ingest`; `connect-src` wird
  nicht erweitert. Das ist keine Bewertung der Zulässigkeit von Telemetrie.

### Formulare und Login-Kompatibilität

Die Quellprüfung fand keine externen `action`-/`formAction`-Ziele. Die Login-
und Anmeldeformulare behandeln `onSubmit` mit `preventDefault` und senden
über die eigene Frontend-Origin. Geprüfte Einstiegspunkte:

- Tenant: `frontend/src/app/[tenant]/page.tsx`.
- Eltern, Schule und Operator: die jeweiligen `login/page.tsx` sowie
  `frontend/src/server/auth/{parent,school,operator,tenant}-config.ts`.
- Öffentliche Anmeldung: `frontend/src/components/enrollment/enrollment-form.tsx`.
- Demo-Portalwechsel: `frontend/src/components/demo/demo-banner.tsx` und
  `frontend/src/app/api/demo/access/handoff/route.ts`. Der Wechsel ist eine
  Navigation, keine externe HTML-Formularübermittlung.

NextAuth `5.0.0-beta.32` sendet im installierten `src/react.tsx` den
`signIn`-POST an seinen jeweiligen eigenen Auth-Pfad und navigiert anschließend
über die zurückgegebene URL. Der Tenant führt außerdem einen Discord-Provider
in seiner Konfiguration; im Anwendungscode wurde kein eigener Discord-
Loginaufruf gefunden. Es wird kein OAuth-Formularziel freigeschaltet.
Ein künftig eingesetzter externer Formular- oder OAuth-Ablauf braucht vorab
eine eigene Kompatibilitätsprüfung. Die bestehende `form-action`-Regel wird
hier nicht verschärft; ein vollständiger Browser-Login ist kein Unit-Test-Nachweis.
Session-Minting, Refresh, MFA und Wrong-Portal-Prüfungen bleiben unverändert.

### Inline-Skripte und verbleibendes XSS-Risiko

`script-src 'self' 'unsafe-inline'` bleibt für die Inline-Bootstrap- und
Hydrierungsskripte von Next.js `16.4.1` erhalten. Einfaches Entfernen ohne
Nonce-/Hash-Konzept würde diese Skripte blockieren. Eine Nonce-Migration
beträfe außerdem Rendering und Caching. Der Slice führt sie nicht durch.
Das ist eine begründete Begrenzung dieser Änderung, keine Aussage, dass
Next.js grundsätzlich `unsafe-inline` erfordert oder das Restrisiko entfällt.
[Next.js dokumentiert beide Betriebsarten und ihre Folgen](https://nextjs.org/docs/app/guides/content-security-policy).

Die Quellprüfung von `frontend/src` fand einen `dangerouslySetInnerHTML`-Sink:
`components/ui/chart.tsx` erzeugt CSS. Die aktuellen produktiven Aufrufer in
Zeiterfassung, Raumhistorie, Feedbackhistorie und Personalübersicht verwenden
feste Chart-Schlüssel und Farben aus `MOTO_COLOR_PALETTE`; es wurde dort kein
direkter Weg von Eingaben zu CSS gefunden. Das ist keine Garantie für spätere
Aufrufer oder transitive Bibliotheken. Rohes HTML, dynamische CSS-Konfiguration
oder neue Skriptquellen erfordern eine erneute Prüfung vor ihrer Nutzung.

Mit `unsafe-inline` bleibt eingeschleustes Inline-JavaScript ausführbar.
React-Ausgabekodierung und sichere DOM-Sinks bleiben daher erforderlich;
die CSP ersetzt sie nicht. Der Slice bestätigt weder vollständigen XSS-Schutz
noch eine abschließende Angemessenheit für alle verarbeiteten Daten.

### Lokale Nachweise und Grenzen

`frontend/src/proxy.test.ts` prüft die drei Direktiven und vorhandenen
Sicherheitsheader für Tenant, Eltern, Schule, Operator, Demo und öffentliche
Anmeldung mit Turnstile. Separate Produktions-/Entwicklungsfälle prüfen
`unsafe-eval` und die unveränderten Inline-Skriptquellen. Die vorhandenen
Turnstile-Tests begrenzen die Ausnahme auf das öffentliche Formular.
Der neue Objektschutz-Test schlug vor der Ergänzung auf allen neun Seiten fehl.
Diese Tests prüfen Header an der Proxy-Schnittstelle, nicht die Durchsetzung
im Browser, nachgelagerte Header oder ausgeschlossene Asset-/Proxy-Pfade.

Die Direktiven folgen der [CSP-Spezifikation](https://www.w3.org/TR/CSP3/#directives).

## Produktiv nachzuweisen: offen

Dieser Slice enthält keine Requests an Produktion oder Staging und keine
Serverzugriffe. Nach dem regulären Deploy prüft ein berechtigter Mensch:

1. Ausgerollten Commit sowie die tatsächlich ausgelieferten CSP- und übrigen
   Sicherheitsheader auf allen oben genannten Portaloberflächen belegen.
   Nachgelagerte Header dürfen die erwartete Policy nicht ersetzen.
2. Login einschließlich MFA, Abmeldung, Einladung/Passwortreset, öffentliche
   Anmeldung mit Turnstile und Demo-Portalwechsel mit dafür vorgesehenen
   Testzugängen prüfen. CSP-Verstöße in der Browserkonsole auswerten.
3. HSTS gesondert am vorgeschalteten Caddy und an der ausgelieferten
   HTTPS-Antwort prüfen. Die Annahme, dass Caddy HSTS setzt, ist in diesem
   Repository nicht belegt; dieser Punkt bleibt bis zum Nachweis offen.
4. Zeitpunkt, Commit, geprüfte Oberflächen und Ergebnis intern festhalten,
   ohne Cookies, Tokens, personenbezogene URLs oder Formularinhalte zu sichern.
   Fehler und verbleibendes Inline-XSS-Risiko bewerten; notwendige Abhilfe
   nicht allein wegen des begrenzten Slices auf später verschieben.

Weitere Kriterien von #3842, etwa Isolation, DB-TLS, privilegierte
Zugriffsprotokolle, Deploy-Rechte und Log-Aufbewahrung, bleiben außerhalb
dieses Slices. Er schließt weder #3842 noch den produktiven Nachweis von AK4 ab.
