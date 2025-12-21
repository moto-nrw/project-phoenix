# Testanleitung: active-service.ts Refactoring

## Übersicht

Diese Anleitung dokumentiert alle refaktorierten Funktionen in `frontend/src/lib/active-service.ts` und wie sie manuell getestet werden können.

**Änderung:** 35 Methoden wurden von inline-Code auf Helper-Funktionen umgestellt. Die Logik ist 100% identisch, nur ausgelagert.

---

## Voraussetzungen

```bash
# Backend starten
docker compose up -d

# Frontend starten
cd frontend && npm run dev
```

**Test-Account:** `admin@example.com` / `Test1234%`

---

## 1. Active Groups

### getActiveGroup
- **Pfad:** `/api/active/groups/{id}`
- **Test:** OGS-Gruppen Seite öffnen → Gruppe anklicken → Details werden geladen
- **UI:** `/ogs-groups` → Gruppe auswählen

### getActiveGroupsByRoom
- **Pfad:** `/api/active/groups/room/{roomId}`
- **Test:** Raum-Übersicht öffnen → Aktive Gruppen im Raum werden angezeigt
- **UI:** `/rooms` → Raum mit aktiver Gruppe auswählen

### getActiveGroupsByGroup
- **Pfad:** `/api/active/groups/group/{groupId}`
- **Test:** Gruppendetails öffnen → Aktive Sessions werden angezeigt
- **UI:** `/groups/{id}`

### getActiveGroupVisits
- **Pfad:** `/api/active/groups/{id}/visits`
- **Test:** Aktive Gruppe öffnen → Schüler-Liste wird geladen
- **UI:** `/ogs-groups` → Gruppe auswählen → Schülerliste sichtbar

### createActiveGroup
- **Pfad:** `POST /api/active/groups`
- **Test:** Neue OGS-Gruppe erstellen
- **UI:** `/ogs-groups` → "Neue Gruppe" Button → Formular ausfüllen → Speichern

### updateActiveGroup
- **Pfad:** `PUT /api/active/groups/{id}`
- **Test:** Bestehende Gruppe bearbeiten (z.B. Raum ändern)
- **UI:** `/ogs-groups` → Gruppe → Bearbeiten → Ändern → Speichern

### deleteActiveGroup
- **Pfad:** `DELETE /api/active/groups/{id}`
- **Test:** Gruppe löschen
- **UI:** `/ogs-groups` → Gruppe → Löschen-Button

### endActiveGroup
- **Pfad:** `POST /api/active/groups/{id}/end`
- **Test:** Aktive Gruppe beenden
- **UI:** `/ogs-groups` → Gruppe → "Beenden" Button

---

## 2. Visits (Besuche)

### getVisit
- **Pfad:** `/api/active/visits/{id}`
- **Test:** Einzelnen Besuch abrufen
- **UI:** Wird intern bei Detailansichten verwendet

### getStudentVisits
- **Pfad:** `/api/active/visits/student/{studentId}`
- **Test:** Schüler-Profil öffnen → Besuchshistorie wird angezeigt
- **UI:** `/students/{id}` → Besuche-Tab

### getVisitsByGroup
- **Pfad:** `/api/active/visits/group/{groupId}`
- **Test:** Gruppen-Besuche abrufen
- **UI:** `/ogs-groups` → Gruppe auswählen

### createVisit
- **Pfad:** `POST /api/active/visits`
- **Test:** Schüler in Gruppe einchecken
- **UI:** `/ogs-groups` → Gruppe → Schüler hinzufügen

### updateVisit
- **Pfad:** `PUT /api/active/visits/{id}`
- **Test:** Besuch aktualisieren
- **UI:** Intern bei Besuchsänderungen

### deleteVisit
- **Pfad:** `DELETE /api/active/visits/{id}`
- **Test:** Besuch löschen
- **UI:** Gruppe → Schüler → Entfernen

### endVisit
- **Pfad:** `POST /api/active/visits/{id}/end`
- **Test:** Schüler auschecken
- **UI:** `/ogs-groups` → Gruppe → Schüler → Auschecken

---

## 3. Supervisors (Betreuer)

### getSupervisor
- **Pfad:** `/api/active/supervisors/{id}`
- **Test:** Einzelnen Betreuer abrufen
- **UI:** Intern verwendet

### getStaffSupervisions
- **Pfad:** `/api/active/supervisors/staff/{staffId}`
- **Test:** Alle Betreuungen eines Mitarbeiters
- **UI:** `/staff/{id}` → Betreuungen-Tab

### getStaffActiveSupervisions
- **Pfad:** `/api/active/supervisors/staff/{staffId}/active`
- **Test:** Aktive Betreuungen eines Mitarbeiters
- **UI:** Dashboard → "Meine aktiven Gruppen"

### getSupervisorsByGroup
- **Pfad:** `/api/active/supervisors/group/{groupId}`
- **Test:** Betreuer einer Gruppe abrufen
- **UI:** `/ogs-groups` → Gruppe → Betreuer-Liste

### createSupervisor
- **Pfad:** `POST /api/active/supervisors`
- **Test:** Betreuer zu Gruppe hinzufügen
- **UI:** `/ogs-groups` → Gruppe → Betreuer hinzufügen

### updateSupervisor
- **Pfad:** `PUT /api/active/supervisors/{id}`
- **Test:** Betreuer-Rolle ändern
- **UI:** Gruppe → Betreuer → Bearbeiten

### deleteSupervisor
- **Pfad:** `DELETE /api/active/supervisors/{id}`
- **Test:** Betreuer entfernen
- **UI:** Gruppe → Betreuer → Entfernen

### endSupervision
- **Pfad:** `POST /api/active/supervisors/{id}/end`
- **Test:** Betreuung beenden
- **UI:** Gruppe → Betreuer → Betreuung beenden

---

## 4. Combined Groups (Kombinierte Gruppen)

### getActiveCombinedGroups
- **Pfad:** `/api/active/combined/active`
- **Test:** Aktive kombinierte Gruppen abrufen
- **UI:** `/ogs-groups` → Kombinierte Gruppen Tab

### getCombinedGroup
- **Pfad:** `/api/active/combined/{id}`
- **Test:** Einzelne kombinierte Gruppe abrufen
- **UI:** Kombinierte Gruppe auswählen

### getCombinedGroupGroups
- **Pfad:** `/api/active/combined/{id}/groups`
- **Test:** Gruppen in einer Kombination abrufen
- **UI:** Kombinierte Gruppe → Enthaltene Gruppen

### createCombinedGroup
- **Pfad:** `POST /api/active/combined`
- **Test:** Neue kombinierte Gruppe erstellen
- **UI:** `/ogs-groups` → "Gruppen kombinieren"

### updateCombinedGroup
- **Pfad:** `PUT /api/active/combined/{id}`
- **Test:** Kombinierte Gruppe bearbeiten
- **UI:** Kombinierte Gruppe → Bearbeiten

### deleteCombinedGroup
- **Pfad:** `DELETE /api/active/combined/{id}`
- **Test:** Kombinierte Gruppe löschen
- **UI:** Kombinierte Gruppe → Löschen

### endCombinedGroup
- **Pfad:** `POST /api/active/combined/{id}/end`
- **Test:** Kombinierte Gruppe beenden
- **UI:** Kombinierte Gruppe → Beenden

---

## 5. Group Mappings

### getGroupMappingsByGroup
- **Pfad:** `/api/active/mappings/group/{groupId}`
- **Test:** Mappings einer Gruppe abrufen
- **UI:** Intern bei kombinierten Gruppen

### getGroupMappingsByCombined
- **Pfad:** `/api/active/mappings/combined/{combinedId}`
- **Test:** Mappings einer Kombination abrufen
- **UI:** Kombinierte Gruppe → Details

### addGroupToCombination
- **Pfad:** `POST /api/active/mappings/add`
- **Test:** Gruppe zu Kombination hinzufügen
- **UI:** Kombinierte Gruppe → Gruppe hinzufügen

### removeGroupFromCombination
- **Pfad:** `POST /api/active/mappings/remove`
- **Test:** Gruppe aus Kombination entfernen
- **UI:** Kombinierte Gruppe → Gruppe entfernen

---

## 6. Analytics

### getAnalyticsCounts
- **Pfad:** `/api/active/analytics/counts`
- **Test:** Dashboard öffnen → Zähler werden angezeigt
- **UI:** `/dashboard` → Statistik-Kacheln

### getRoomUtilization
- **Pfad:** `/api/active/analytics/room/{roomId}/utilization`
- **Test:** Raum-Auslastung anzeigen
- **UI:** `/rooms/{id}` → Auslastungs-Anzeige

### getStudentAttendance
- **Pfad:** `/api/active/analytics/student/{studentId}/attendance`
- **Test:** Schüler-Anwesenheit anzeigen
- **UI:** `/students/{id}` → Anwesenheits-Statistik

---

## 7. Claiming

### claimActiveGroup
- **Pfad:** `POST /api/active/groups/{groupId}/claim`
- **Test:** Gruppe beanspruchen
- **UI:** `/my-room` → Verfügbare Gruppen → Beanspruchen

---

## Automatisierte Tests (Bruno)

```bash
cd bruno

# Alle Tests ausführen
bru run --env Local 0*.bru

# Spezifische Tests
bru run --env Local 05-sessions.bru    # Session-Tests
bru run --env Local 06-checkins.bru    # Check-in/out Tests
```

---

## Schnell-Test Checkliste

| Bereich | Test | Erwartet |
|---------|------|----------|
| Dashboard | Öffnen | Statistiken laden |
| OGS-Gruppen | Liste laden | Gruppen werden angezeigt |
| OGS-Gruppe | Details öffnen | Schüler + Betreuer laden |
| Schüler | Einchecken | Schüler erscheint in Liste |
| Schüler | Auschecken | Schüler wird entfernt |
| Betreuer | Hinzufügen | Betreuer erscheint |
| Gruppe | Beenden | Status ändert sich |
| Kombinieren | Gruppen kombinieren | Kombination erstellt |

---

## Fehlersuche

Falls Fehler auftreten:

1. **Browser Console öffnen** (F12) → Netzwerk-Tab
2. **Fehler-Format prüfen:**
   - `{Operation} error: {status}` in Console
   - `{Operation} failed: {status}` als Error-Message
3. **Backend-Logs prüfen:** `docker compose logs -f server`

---

## Betroffene Datei

```
frontend/src/lib/active-service.ts
```

**Zeilen:** 2055 → 1201 (854 Zeilen weniger, -42%)
