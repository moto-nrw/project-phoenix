# Pull Request

## TL;DR: Was ändert sich für Nutzer?
<!-- Pflicht, 1–3 Sätze in einfacher Sprache aus Sicht der Betroffenen:
     Wer (Betreuer, Eltern, Schulleitung, Kiosk) merkt was, wo im Produkt?
     Keine Dateien, Module oder Klassennamen.
     Ohne sichtbare Wirkung: „Keine Änderung für Nutzer“ plus Halbsatz warum. -->

## Description
<!-- Explain the changes you've made and why they're needed -->

## Backend-Architektur
<!-- Pflicht in jeder PR-Beschreibung, auch bei Erstellung per CLI und bei Updates.
     Ohne Backend-Bezug: „Nicht betroffen“ plus kurze Begründung.
     Bei Backend-Änderungen konkret ausfüllen, mit betroffenen Codepfaden als Beleg:
     - Zielarchitektur: betroffene Owner, öffentliche Capabilities und Paketrollen;
       passende Prinzip-ADRs aus backend/architecture/README.md und Einträge in
       backend/architecture/policy.json nennen und erklären, warum die Änderung dort liegt.
     - Grenzen: relevante Abhängigkeiten/Ports, Cross-Owner-Lese- und Schreibzugriffe,
       Transaktionen und Composition erklären. Nicht betroffene Grenzen kurz benennen.
     - Drift: Policy-, Baseline- und Composition-Inventar-Änderungen sowie Vorher-/Nachher-
       Zahlen berührter Shrink-only-Flächen nennen (nur gleichbleibend oder sinkend).
       Abweichungen mit ADR-Entscheidung belegen; offene Architekturfragen offen ausweisen.
     - Nachweis: tatsächlich ausgeführte Architekturchecks mit Befehl und Ergebnis;
       nicht ausgeführte oder blockierte Checks ausdrücklich nennen.
     Ein grüner Check allein ersetzt die Architekturbegründung nicht.
     Maßgeblich: backend/CLAUDE.md, Abschnitt „Architecture“. -->

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update
- [ ] Configuration change
- [ ] Security improvement
- [ ] Refactoring
- [ ] Performance improvement
- [ ] Test addition/modification
- [ ] CI/CD improvement

## Related Issues
<!-- List any related issues using the following syntax:
- Fixes #123
- Addresses #456
- Related to #789
-->

## Testing
<!-- Describe how you tested your changes -->
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed
- [ ] Testing documentation updated

## Security Checklist
<!-- This section is mandatory for all PRs -->
- [ ] I have followed the [security guidelines](../docs/security.md)
- [ ] No sensitive information is committed (secrets, credentials, certificates)
- [ ] Environment variables are properly managed with templates
- [ ] Code does not contain hardcoded secrets
- [ ] No potential security vulnerabilities introduced

## Screenshots or Examples
<!-- Add screenshots or code examples if applicable -->

## Missverständnis-Check
<!-- Mandatory for user-visible changes (tenant portal, parents portal, kiosk, e-mails, help guide).
     Checklist: .claude/rules/verstaendlichkeit.md — name the blocks you checked and what you changed.
     Write "nicht user-sichtbar" if it does not apply. -->

## Checklist
- [ ] My code follows the project's style guidelines
- [ ] I have performed a self-review of my code
- [ ] I have commented my code, especially in hard-to-understand areas
- [ ] I have updated the documentation as needed
- [ ] If this PR changes a user-facing flow, I updated the affected in-app help articles (`frontend/src/components/help/help-content.ts`) — or it doesn't apply (backend-only, operator-portal-only, or pure-styling change)
- [ ] User-visible change: I ran the Verständlichkeit checklist (`.claude/rules/verstaendlichkeit.md`) against the running app and recorded the result above — or it doesn't apply
- [ ] All tests are passing
- [ ] My changes generate no new warnings or errors
- [ ] I have checked for and resolved any merge conflicts

## Additional Notes
<!-- Add any other context about the PR here -->
