## 1. Implementation
- [x] 1.1 Add `ToastProvider` and `useToast()` hook (global stack, theme, a11y)
- [x] 1.2 Refactor `SimpleAlert` styles into reusable ToastItem (no API break yet)
- [x] 1.3 Wire `ToastProvider` in `frontend/src/app/providers.tsx`
- [x] 1.4 Add de-duplication (2s window) and stacking (max 3)
- [x] 1.5 Respect `prefers-reduced-motion` (disable transitions/progress)

## 2. Migrations (Pages/Flows)
- [x] 2.1 Database list/detail flows → `toast.success` (create/update/delete)
  - [x] frontend/src/components/ui/database/database-page.tsx:681
- [x] 2.2 Profile actions → `toast.success`
  - [x] frontend/src/app/profile/page.tsx:512 (password + profile + avatar)
- [x] 2.3 Settings → `toast.success`
  - [x] frontend/src/app/settings/page.tsx:749
- [x] 2.4 Devices/Rooms → `toast.success`
  - [x] frontend/src/app/database/devices/page.tsx:299
  - [x] frontend/src/app/database/rooms/page.tsx:405
- [x] 2.5 Modals (roles/permissions/teachers/activities) → `toast.success`
  - [x] frontend/src/components/teachers/teacher-permission-management-modal.tsx:390
  - [x] frontend/src/components/teachers/teacher-role-management-modal.tsx
  - [x] frontend/src/components/auth/role-permission-management-modal.tsx
  - [x] frontend/src/components/activities/quick-create-modal.tsx
  - [x] frontend/src/components/activities/time-management-modal.tsx

## 3. Cleanup
- [x] 3.1 Remove page-level `showSuccessAlert`/`successMessage` state where replaced
- [x] 3.2 Remove or adapt `AlertContext` if superseded by `ToastProvider`
- [x] 3.3 Ensure `getDbOperationMessage` is reused for CRUD toasts
- [x] 3.4 Standardize toast colors to moto brand hex (success/info/warn/error)

## 4. Validation
- [x] 4.1 Desktop: bottom-right placement, stacking, hover pause, close button
- [x] 4.2 Mobile: full-width near bottom with safe-area, large tap targets
- [x] 4.3 A11y: `role="status"`, `aria-live="polite"`, `aria-atomic="true"`
- [x] 4.4 Motion: transitions disabled under reduced motion
- [x] 4.5 Dedupe: identical messages within 2s only once

## 5. OpenSpec
- [x] 5.1 Draft spec deltas under `ui-notifications`
- [x] 5.2 Validate: `openspec validate update-success-notifications --strict`
- [x] 5.3 Share proposal for approval before implementation
