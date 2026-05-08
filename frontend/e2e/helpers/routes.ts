/**
 * Centralised URL helpers for the E2E suite. Specs MUST go through these
 * instead of hardcoding paths in `page.goto(...)` calls.
 *
 * Why: CLAUDE.md (URL & Route Conventions) commits to migrating legacy
 * snake_case routes (`feedback_history`, `mensa_history`) to kebab-case
 * "when touched". Without a single source of truth, every such migration
 * means grep-and-replace across the spec files — and any miss silently
 * breaks tests at runtime instead of compile-time.
 *
 * Convention: paths are returned as tenant-relative strings. Specs compose
 * them with the `app` fixture (e.g. `page.goto(app.primary(routes.root))`)
 * so tenant/subdomain semantics live in Playwright fixtures, not config.
 *
 * Add new helpers here whenever a spec needs a new page; do not let
 * `page.goto("/some/new/path")` creep back into spec files.
 */

/** Tenant root — the dashboard for admins, /myroom-equivalent for staff. */
export const root = "/";

/** Admin: list of students under Datenbank → Schüler. */
export const studentsList = "/database/students";

/**
 * Admin: master/detail student page with one student preselected. The
 * page reads the `student` query param and opens the Stammdaten tab for
 * that id — useful for tests that need to land directly on a specific
 * student without clicking through the list.
 */
export const studentDetail = (studentId: number | string): string =>
  `/database/students?student=${studentId}`;

/** Admin: list of groups under Datenbank → Gruppen. */
export const groupsList = "/database/groups";

/** Staff: OGS groups overview. */
export const ogsGroups = "/ogs-groups";
