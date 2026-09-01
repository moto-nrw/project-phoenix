# Gestapelte Modals im Phoenix-Frontend — Audit (#2774)

Stand: 2026-08-31, Basis `origin/development` (1b6aeaa34). Erfasst wurden alle
Portale (Tenant, Operator, Eltern, Schule) und die gemeinsam genutzten
Komponenten. Gesucht wurde nach Fällen, in denen ein zweiter eigenständiger
Dialog rendert, während ein erster Dialog offen bleibt — über die
Kit-Primitives (`Modal`, `ConfirmationModal`, `FormModal`,
`ConfirmDeleteModal`, `ChoiceModal`, `Drawer`, `SlideOver`,
`useDeleteConfirmation`) und über selbstgebaute `fixed inset-0`-Overlays.

Kleine aufgabenbezogene Overlays (Datepicker, Auswahlmenüs, OverflowMenu,
Tooltips) sind laut Issue ausdrücklich kein Stapeln und wurden nicht erfasst.

## Technischer Rahmen

- Alle Dialog-Primitives liegen auf `z-[9999]`; zwei gleichzeitig sichtbare
  Modals ordnen sich nur über die DOM-Reihenfolge. `SlideOver` (z-40/50) und
  `Drawer` (z-50) liegen darunter, ein Modal darüber zeichnet also korrekt.
- Jedes offene Kit-Modal registriert einen eigenen document-weiten
  Escape-Listener. Bei zwei gleichzeitig offenen Modals schließt ein
  Escape-Druck beide auf einmal — ein Grund mehr, echtes Modal-über-Modal zu
  vermeiden.
- Fokus-Verschachtelung funktioniert (Radix `FocusScope` pausiert den
  Eltern-Scope), Scroll-Lock ist über `useScrollLock` idempotent.
- Das etablierte Zielmuster im Code ist **„A ausblenden, solange B offen
  ist“**: der äußere Dialog bekommt `isOpen={isOpen && !bOffen}`, der zweite
  Dialog ist ein Geschwister-Element (nicht in den Children des ersten).
  Referenzen: `components/staff/shift-edit-modal.tsx`,
  `components/staff/shift-move-dialog.tsx`,
  `components/students/care-plan-editor-modal.tsx`,
  `components/timetable/instance-detail-modal.tsx`.

## Gewollt (bleibt, dokumentierte Produktentscheidung)

| # | Ort | Stapel | Begründung |
|---|-----|--------|------------|
| G1 | Datenbank-Seiten mobil (`components/database/master-detail-layout.tsx`) | Detail-`Drawer` + Seiten-Modals (Bearbeiten, Löschen, Rollen, MFA, …) | Bewusstes Muster aus #1358: der Drawer ist mobiler Navigations-Container, kein Dialog. `onInteractOutside`/`onEscapeKeyDown` sind über `isModalOpen` geschützt, der Modal-Context zählt offene Ebenen. |
| G2 | Betreuungsplan (`/betreuungsplan`, `/calendar?tab=schule`): `TimetableEventModal` (SlideOver) | Bestätigungen/Auswahl-Dialoge über dem SlideOver (Löschen, Serien-Scope, Schließtag, Einzelanpassungen, Quell-Angebot) und `CategoryManageModal` | SlideOver liegt auf z-50 unter den Modals, `onInteractOutside` ist während offener Modals geschützt (#1358). Die Dialoge gehören zur selben Aufgabe (Termin speichern/löschen); `CategoryManageModal` blendet sich selbst aus, solange seine Archiv-Bestätigung offen ist. |
| G3 | Betreuungsplan: `StaffPoolSlideOver` | Verschieben/Zuweisen-`ConfirmationModal` über dem SlideOver | Gleiche Aufgabe, SlideOver ist während der Bestätigung über `dismissible` gesperrt. |

Eigenständiges Modal über eigenständigem Modal kommt in G1–G3 nicht vor —
der untere Layer ist immer SlideOver/Drawer.

## Ungewollt (behoben in diesem PR)

Zielablauf überall: das bestehende Repo-Idiom „A ausblenden, solange B offen
ist“. Der Ablauf bleibt identisch (B schließen bringt A zurück), aber es ist
immer nur ein eigenständiger Dialog sichtbar; Escape/Backdrop/Fokus wirken
eindeutig auf den sichtbaren Dialog.

| # | Route | Dialog A | Dialog B | Auslöser |
|---|-------|----------|----------|----------|
| U1 | `/betreuungsplan` | `InstanceDetailModal` | `CalendarPeriodModal` | „Wiederholen“ ohne Planungszeitraum: `suspended`-Guard deckte `periodModalOpen` nicht ab |
| U2 | `/database/personal`, Operator Konten/Organisationen | `CaregiverCapabilityModal` (FormModal) | `CaregiverBlockerResolutionModal` (FormModal) | „Zuordnungen auflösen“ im Blocker-Abschnitt; B war in A verschachtelt |
| U3 | Operator `/accounts` | `AccountTenantAccessModal` (FormModal) | Entzugs-`ConfirmationModal` | „Zugang entziehen“ pro Schule |
| U4 | `/database/students` | `StudentCreateModal` („Neues Kind“) | `GuardianFormModal` | „Neu anlegen“ bei Erziehungsberechtigten |
| U5 | `/database/students` | `StudentCreateModal` | `CareWeeklyPlanModal` | Betreuungszeiten-Wochenplan |
| U6 | `/students/[id]` | `PlannedStatusDaysModal` (FormModal) | `ConfirmDeleteModal` | Teilentschuldigung löschen; B war in A verschachtelt |
| U7 | Eltern-Portal `/children/[id]` | `OfferingChangeRequestModal` | Abmelde-`ConfirmationModal` | Änderung entfernt alle Betreuungstage; B war in A verschachtelt |
| U8 | `/admin/enrollments/[id]` + Kind-Detail „Anmeldungen“ | `ChildOfferingAdjustment` (bespoke `fixed inset-0`-Overlay) | Abmelde-`ConfirmationModal` | Speichern mit `complete_withdrawal_confirmation_required` |
| U9 | `/database/grade-transitions` | `TransitionPreviewModal` (`isOpen` war Literal `true`) | Anwenden-`ConfirmationModal` | „Anwenden“ |
| U10 | `/database/grade-transitions` | `GraduatesModal` (`isOpen` war Literal `true`) | Endgültig-löschen-`ConfirmationModal` | „Endgültig löschen“ |

## Geprüft und sauber (Auswahl der verdächtigen Kandidaten)

Diese Stellen sahen nach Stapeln aus, nutzen aber bereits das
Ausblende-Idiom oder öffnen ihre Dialoge strikt nacheinander bzw. gegenseitig
ausschließend:

- `shift-edit-modal`, `shift-move-dialog`, `shift-type-manage-modal`,
  `category-manage-modal` (intern), `instance-detail-modal` (eigene
  Bestätigungen), `care-plan-editor-modal` — Ausblende-Idiom.
- `abwesenheiten-tab`, `stammdaten-section-modals`, `stundenkonto-panel`,
  `time-tracking/page`, `leave-requests-card`, `dienstplan-view`,
  `vertretung-view`, `substitutions/page`, `calendar/page` — Seiten-Sibling-
  Modals mit disjunkten Triggern.
- `guardians-panel` (Eltern), `withdrawal-cards`, `students/search`,
  `students/[id]` (Krank/Entschuldigt-Verzweigung), `care-schedule-manager`,
  `grade-transitions-manager` (Editor↔Preview-Übergaben), Enrollment-Editoren,
  `parent-announcements`, `meal-plan`, `info-displays` (Token-Modal öffnet
  erst nach Schließen des Formulars), `class-list` (Union-State),
  Operator-Provisioning (Zwei-Zweig-Modals), `settings-field`,
  `files-page`, Schul-Portal (ein flaches Modal), Raum-Detail
  (SlideOver/Drawer ohne inneren Dialog).
- `LogoutModal`/`PasswordChangeModal`/`PasswordResetModal` werden nur aus
  Seiten-/Navigations-Chrome geöffnet.

Einziges selbstgebautes Vollbild-Overlay außerhalb des Kits in den geprüften
Flächen: `admin-enrollment-detail.tsx` (`ChildOfferingAdjustment`, U8);
daneben Operator `soft-delete-shared.tsx`, `persons/page.tsx`,
`unregistered-tags/page.tsx` und `mfa-admin-override-modal.tsx` — alle flach,
kein Stapeln.
