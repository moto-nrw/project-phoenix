# RALPH LOOP: Coverage auf 80%+

Du bist in einer Loop. Pro Iteration: Tests fuer EINE Datei schreiben, committen, beenden.

## Kontext

SonarQube "Coverage on New Code" = nur Zeilen die in diesem Branch neu sind.
Diese Dateien haben 0% Coverage und brauchen Tests.
Arbeite die Liste von oben nach unten ab (groesste Impact zuerst).

## Arbeitsverzeichnisse

- betterauth: /Users/yonnock/Developer/moto/project-phoenix/.worktrees/ralph-coverage/betterauth
- frontend: /Users/yonnock/Developer/moto/project-phoenix/.worktrees/ralph-coverage/frontend

## Uncovered New Code (von SonarQube)

### betterauth (329 + 31 + 11 + 6 + 7 = 384 lines)
- [x] src/index.ts (329 lines) - WICHTIGSTE DATEI
- [x] src/email.ts (31 lines) - has email.test.ts
- [x] src/auth.ts (11 lines) - excluded from coverage (BetterAuth config)
- [x] src/permissions.ts (6 lines) - has permissions.test.ts
- [x] src/test/setup.ts (7 lines) - test setup file (not production code)

### frontend/src/lib (360 lines)
- [x] lib/route-wrapper.ts (61 lines) - has route-wrapper.test.ts
- [x] lib/email-validation.ts (56 lines) - has email-validation.test.ts (80 tests)
- [x] lib/admin-api.ts (42 lines) - has admin-api.test.ts (44 tests)
- [x] lib/auth-client.ts (23 lines) - has auth-client.test.ts
- [x] lib/supervision-context.tsx (23 lines) - has supervision-context.test.tsx (33 tests)
- [x] lib/slug-validation.ts (21 lines) - has slug-validation.test.ts
- [x] lib/api-helpers.ts (20 lines) - has api-helpers.test.ts
- [ ] lib/admin-auth.ts (17 lines)
- [ ] lib/tenant-context.ts (15 lines)
- [ ] lib/profile-context.tsx (14 lines)
- [ ] lib/redirect-utils.ts (11 lines)
- [ ] lib/hooks/ (11 lines)
- [ ] lib/api-client.ts (8 lines)
- [x] lib/api.ts (5 lines) - has api.test.ts
- [x] lib/auth-api.ts (4 lines) - has auth-api.test.ts
- [ ] lib/swr/ (4 lines)
- [x] lib/teacher-api.ts (4 lines) - has teacher-api.test.ts
- [x] lib/auth-service.ts (3 lines) - has auth-service.test.ts
- [ ] lib/file-upload-wrapper.ts (3 lines)
- [ ] lib/password-validation.ts (3 lines)
- [ ] lib/usercontext-context.tsx (3 lines)
- [ ] lib/database/ (2 lines)
- [x] lib/student-api.ts (2 lines) - has student-api.test.ts
- [x] lib/activity-api.ts (1 line) - has activity-api.test.ts
- [x] lib/api-helpers.server.ts (1 line) - has api-helpers.server.test.ts
- [ ] lib/auth-utils.ts (1 line)
- [x] lib/checkin-api.ts (1 line) - has checkin-api.test.ts
- [x] lib/rooms-api.ts (1 line) - has rooms-api.test.ts

### frontend/src/components (473 lines)
- [ ] components/auth/ (201 lines)
- [ ] components/admin/ (144 lines)
- [ ] components/console/ (105 lines)
- [ ] components/ui/ (15 lines)
- [ ] components/dashboard/ (5 lines)
- [ ] components/auth-wrapper.tsx (2 lines)
- [ ] components/teachers/ (1 line)

### frontend/src/app (745 lines)
- [ ] app/api/ (249 lines)
- [ ] app/(auth)/ (162 lines)
- [ ] app/(public)/ (125 lines)
- [ ] app/login/ (112 lines)
- [ ] app/database/ (27 lines)
- [ ] app/page.tsx (23 lines)
- [ ] app/invitations/ (9 lines)
- [ ] app/dashboard/ (8 lines)
- [ ] app/settings/ (6 lines)
- [ ] app/ogs-groups/ (5 lines)
- [ ] app/rooms/ (5 lines)
- [ ] app/students/ (5 lines)
- [ ] app/active-supervisions/ (3 lines)
- [ ] app/staff/ (3 lines)
- [ ] app/substitutions/ (3 lines)

## Iteration

### 1. Naechste Datei waehlen

Finde die erste Datei die noch [ ] (unchecked) ist.
Wenn alle [x] markiert sind: Gib aus `RALPH_DONE` und beende.

### 2. Tests schreiben

1. Lies die Quelldatei komplett
2. Identifiziere alle Funktionen/Exports
3. Erstelle/erweitere `{filename}.test.ts` mit Tests fuer ALLE uncovered lines
4. Mocke externe Dependencies (fetch, next/navigation, etc.)

### 3. Verifizieren

```bash
# betterauth
cd /Users/yonnock/Developer/moto/project-phoenix/.worktrees/ralph-coverage/betterauth
pnpm test && pnpm run check && pnpm knip
pnpm test  # Zweimal!

# frontend
cd /Users/yonnock/Developer/moto/project-phoenix/.worktrees/ralph-coverage/frontend
pnpm test -- --run && pnpm run check && pnpm knip
pnpm test -- --run  # Zweimal!
```

Beide Laeufe muessen durchgehen.

### 4. Committen

```bash
cd /Users/yonnock/Developer/moto/project-phoenix/.worktrees/ralph-coverage
git add -A
git commit -m "test: add coverage for {datei}"
```

### 5. TASK.md updaten

Markiere die Datei als erledigt: `- [ ]` wird zu `- [x]`

### 6. Beenden

```
ITERATION COMPLETE
Datei: {pfad}
Lines covered: {anzahl}
```

## Abbruch

Wenn ALLE Dateien mit [x] markiert sind:
Gib EXAKT aus: RALPH_DONE

## STARTE JETZT

Finde die erste unchecked Datei und schreibe Tests.
