// Bauarten ratchet (BAUARTEN-SPEC.md, Abschnitt „Ratschen“).
//
// Two rules; a production match beyond the tolerated remainder fails
// `pnpm run check`:
//
//   bauart/one-delete-confirm — deletion is confirmed by ConfirmDeleteModal
//                               only (Bauart 2 Regel 6, issue #3110). The
//                               rule looks at the label of the PRIMARY
//                               ACTION, because that is what tells the user
//                               what the dialog does: `confirmText` on
//                               ConfirmationModal, `title` on a hand-built
//                               Modal / ChoiceModal / FormModal, and any
//                               `window.confirm` call. A dialog whose action
//                               is a state change with a removal side effect
//                               („Änderung speichern“, „Trotzdem speichern“)
//                               is not a deletion and passes. Hard-zero.
//
//   bauart/no-row-action-buttons — row actions live in the row's kebab
//                               (`OverflowMenu` from ui/page-header) only
//                               (Bauart 1 Regel 4, issue #3111). The rule
//                               flags a `Button` / `<button>` rendered per
//                               list item (inside a `.map(…)` callback or a
//                               DataTable column `render`) whose accessible
//                               name is an object action: bearbeiten,
//                               löschen, entfernen, archivieren,
//                               wiederherstellen, duplizieren, umbenennen,
//                               veröffentlichen, kopieren. Reorder arrows
//                               („nach oben“) are list actions and pass; an
//                               icon-only „… entfernen“ button is the chip
//                               remover of a form value list and passes.
//                               Shrink-only per-file baseline
//                               (ROW_ACTION_BASELINE) for the remainder the
//                               issue defers to follow-up PRs. Scope is the
//                               tenant portal: operator, parents and school
//                               portal files are exempt (the spec does not
//                               cover them).
//
// Files under src/components/ui/ are exempt (ConfirmDeleteModal is itself
// built on Modal), as are tests and stories.

const UI_KIT_DIR_RE = /(?:^|\/)src\/components\/ui\//;
const EXEMPT_FILE_RE = /(?:\.(?:test|stories)\.)|(?:\.d\.ts$)/;
// BAUARTEN-SPEC: „Nicht betroffen: Eltern-Portal, Schul-Portal,
// Operator-Portal.“
const OTHER_PORTAL_RE =
  /(?:^|\/)src\/(?:app\/(?:operator|parents|school)|components\/(?:operator|parent|school))\//;

// Word-bounded for letters including umlauts: JS `\b` only knows ASCII, so
// „gelöscht“ / „Löschung“ stay out and „Löschen“, „Ja, löschen“,
// „Endgültig entfernen“, „Löschen prüfen“ match.
const DELETE_LABEL_RE = /(?:^|\P{L})(?:löschen|entfernen)(?:\P{L}|$)/iu;

const CONFIRMATION_MODAL = "ConfirmationModal";
const CUSTOM_MODALS = new Set(["Modal", "ChoiceModal", "FormModal"]);

function fileName(context) {
  return String(context.physicalFilename ?? context.filename ?? "").replaceAll(
    "\\",
    "/",
  );
}

function isExempt(context) {
  const name = fileName(context);
  return UI_KIT_DIR_RE.test(name) || EXEMPT_FILE_RE.test(name);
}

/** Static string chunks reachable in an attribute value (literal, template,
 *  both branches of a conditional, …). Mirrors the ui-kit collector. */
function collectStaticStrings(node, chunks, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return;
  seen.add(node);

  if (node.type === "Literal" && typeof node.value === "string") {
    chunks.push(node.value);
    return;
  }
  if (node.type === "TemplateElement") {
    chunks.push(node.value.raw);
    return;
  }
  if (node.type === "JSXText") {
    chunks.push(node.value);
    return;
  }

  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      for (const child of value) collectStaticStrings(child, chunks, seen);
    } else {
      collectStaticStrings(value, chunks, seen);
    }
  }
}

function jsxAttribute(openingElement, name) {
  return openingElement.attributes.find(
    (attribute) =>
      attribute.type === "JSXAttribute" &&
      attribute.name.type === "JSXIdentifier" &&
      attribute.name.name === name,
  );
}

function attributeMentionsDeletion(attribute) {
  if (!attribute?.value) return false;
  const chunks = [];
  collectStaticStrings(attribute.value, chunks);
  return chunks.some((chunk) => DELETE_LABEL_RE.test(chunk));
}

function jsxName(node) {
  return node?.type === "JSXIdentifier" ? node.name : null;
}

const oneDeleteConfirm = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Deletion is confirmed by ConfirmDeleteModal only: no window.confirm, no ConfirmationModal and no hand-built modal for Löschen.",
    },
    messages: {
      windowConfirm:
        "Löschen bestätigt portalweit ConfirmDeleteModal (BAUARTEN-SPEC Bauart 2 Regel 6), nicht window.confirm.",
      confirmationModal:
        "ConfirmationModal ist für Zustandswechsel. Eine Löschung („{{label}}“) bestätigt ConfirmDeleteModal (BAUARTEN-SPEC Bauart 2 Regel 6); für Unwiderrufliches mit Datenverlust gate textConfirm, sonst twoStep.",
      customModal:
        "Kein eigenes Löschmodal je Domäne („{{label}}“): Löschen bestätigt ConfirmDeleteModal (BAUARTEN-SPEC Bauart 2 Regel 6). Eine Scope-Wahl (Serie/Einzeltermin) gehört in dessen `scope`-Slot.",
    },
    schema: [],
  },
  create(context) {
    if (isExempt(context)) return {};

    return {
      CallExpression(node) {
        const callee = node.callee;
        if (
          callee?.type === "MemberExpression" &&
          callee.object?.type === "Identifier" &&
          (callee.object.name === "window" ||
            callee.object.name === "globalThis") &&
          callee.property?.type === "Identifier" &&
          callee.property.name === "confirm"
        ) {
          context.report({ node, messageId: "windowConfirm" });
        }
      },
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name === CONFIRMATION_MODAL) {
          const attribute = jsxAttribute(node, "confirmText");
          if (attributeMentionsDeletion(attribute)) {
            const chunks = [];
            collectStaticStrings(attribute.value, chunks);
            context.report({
              node,
              messageId: "confirmationModal",
              data: {
                label: chunks.find((chunk) => DELETE_LABEL_RE.test(chunk)),
              },
            });
          }
          return;
        }
        if (CUSTOM_MODALS.has(name)) {
          const attribute = jsxAttribute(node, "title");
          if (attributeMentionsDeletion(attribute)) {
            const chunks = [];
            collectStaticStrings(attribute.value, chunks);
            context.report({
              node,
              messageId: "customModal",
              data: {
                label: chunks.find((chunk) => DELETE_LABEL_RE.test(chunk)),
              },
            });
          }
        }
      },
    };
  },
};

// --- bauart/no-row-action-buttons -------------------------------------------

/** Repo-relative posix path ("src/…"), or the raw name when unavailable. */
function fileKey(context) {
  const posix = fileName(context);
  const idx = posix.lastIndexOf("/src/");
  return idx === -1 ? posix : posix.slice(idx + 1);
}

// Object actions that belong in the row's kebab. Word-bounded incl. umlauts
// (see DELETE_LABEL_RE): „Jahrgang 1 archivieren“, „Löscht…“ does NOT match
// (different word), „Endgültig löschen“ and „Link kopieren“ do.
const ROW_ACTION_LABEL_RE =
  /(?:^|\P{L})(?:bearbeiten|löschen|entfernen|archivieren|wiederherstellen|duplizieren|umbenennen|veröffentlichen|kopieren)(?:\P{L}|$)/iu;
const CHIP_REMOVE_RE = /(?:^|\P{L})entfernen(?:\P{L}|$)/iu;

// Shrink-only per-file tolerance: matches that were on the books when the
// rule landed and that issue #3111 defers to follow-up PRs, plus the
// form-internal text buttons the rule cannot tell apart from a row action
// („Entfernen“ on a pending upload, on a guardian entry of the create form).
// A file may only ever go DOWN here; a new match in a file without an entry
// fails the check. Keys are repo-relative posix paths under src/.
const ROW_ACTION_BASELINE = new Map(
  Object.entries({
    // Noch umzuziehen (#3111, Folge-PRs): Objektaktionen je Zeile.
    "src/app/[tenant]/(protected)/database/students/class-list/page.tsx": 2,
    "src/app/[tenant]/(protected)/database/students/ended-care/page.tsx": 1,
    "src/components/admin/pending-invitations-list.tsx": 1,
    "src/components/database/grade-transitions/grade-transitions-manager.tsx": 2,
    "src/components/groups/group-transfer-modal.tsx": 1,
    "src/components/guardians/guardian-list.tsx": 1,
    "src/components/planning/calendar-periods-editor.tsx": 1,
    "src/components/settings/trusted-devices-section.tsx": 1,
    "src/components/staff/staff-session-table.tsx": 2,
    "src/components/staff/stundenkonto-panel.tsx": 1,
    "src/components/students/class-arrival-exception-panel.tsx": 1,
    "src/components/teachers/caregiver-blocker-resolution-panel.tsx": 3,
    "src/components/timetable/period-switcher-dropdown.tsx": 1,
    // Formular-intern (Eintrag eines Formularwerts, kein gespeichertes
    // Objekt): bleibt, die Regel kann Textknöpfe nicht von Zeilenaktionen
    // unterscheiden.
    "src/app/[tenant]/(protected)/parent-announcements/page.tsx": 1,
    "src/components/enrollment/care-offerings-editor.tsx": 1,
    "src/components/enrollment/enrollment-form-editor.tsx": 1,
    "src/components/guardians/guardian-form-modal.tsx": 1,
    "src/components/staff/shift-edit-modal.tsx": 1,
    "src/components/timetable/substitution-slide-over.tsx": 1,
  }),
);

/** Text a JSX element renders: its own text nodes, static strings in
 *  expression children, and the text of nested elements — never the
 *  attributes of nested elements (an icon's className is not a label). */
function collectChildText(element, chunks) {
  for (const child of element?.children ?? []) {
    collectRenderedText(child, chunks);
  }
}

function collectRenderedText(node, chunks, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return;
  seen.add(node);
  if (
    (node.type === "Literal" && typeof node.value === "string") ||
    node.type === "JSXText"
  ) {
    chunks.push(node.value);
    return;
  }
  if (node.type === "TemplateElement") {
    chunks.push(node.value.raw);
    return;
  }
  // Attributes of nested elements are props, not rendered text.
  if (node.type === "JSXOpeningElement" || node.type === "JSXAttribute") return;
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      for (const child of value) collectRenderedText(child, chunks, seen);
    } else {
      collectRenderedText(value, chunks, seen);
    }
  }
}

function staticText(node) {
  if (!node) return "";
  const chunks = [];
  collectStaticStrings(node, chunks);
  return chunks.join(" ").replaceAll(/\s+/g, " ").trim();
}

/** Text the browser would use as the accessible name, in WAI order:
 *  aria-label, then text content, then title. `iconOnly` says whether the
 *  button renders any static text of its own. */
function accessibleName(openingElement) {
  const chunks = [];
  collectChildText(openingElement.parent, chunks);
  const childText = chunks.join(" ").replaceAll(/\s+/g, " ").trim();
  const iconOnly = childText === "";
  const aria = staticText(jsxAttribute(openingElement, "aria-label")?.value);
  if (aria) return { text: aria, iconOnly };
  if (childText) return { text: childText, iconOnly };
  return {
    text: staticText(jsxAttribute(openingElement, "title")?.value),
    iconOnly,
  };
}

/** True when the node sits inside a callback rendered once per list item:
 *  an argument of `.map(…)` or the `render` / `cell` function of a column. */
function isPerItemRender(node) {
  let current = node.parent;
  while (current) {
    if (
      current.type === "ArrowFunctionExpression" ||
      current.type === "FunctionExpression"
    ) {
      const owner = current.parent;
      if (
        owner?.type === "CallExpression" &&
        owner.callee?.type === "MemberExpression" &&
        owner.callee.property?.type === "Identifier" &&
        owner.callee.property.name === "map" &&
        owner.arguments.includes(current)
      ) {
        return true;
      }
      if (
        owner?.type === "Property" &&
        owner.key?.type === "Identifier" &&
        (owner.key.name === "render" || owner.key.name === "cell")
      ) {
        return true;
      }
    }
    current = current.parent;
  }
  return false;
}

const noRowActionButtons = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Row actions (Bearbeiten, Löschen, Archivieren, …) live in the row's OverflowMenu, not as an icon row or text buttons per list item.",
    },
    messages: {
      rowAction:
        "Zeilenaktion „{{label}}“ gehört in den Kebab der Zeile (OverflowMenu aus ui/page-header), nicht als Icon-Reihe oder Textknopf je Zeile (BAUARTEN-SPEC Bauart 1 Regel 4, #3111). Umsortieren (↑/↓) und Chip-Entfernen in Formularen sind keine Objektaktionen. Die Baseline in scripts/oxlint-plugin-bauart.mjs ist shrink-only.",
    },
    schema: [],
  },
  create(context) {
    if (isExempt(context)) return {};
    const key = fileKey(context);
    if (OTHER_PORTAL_RE.test(key)) return {};
    const tolerated = ROW_ACTION_BASELINE.get(key) ?? 0;
    let seen = 0;

    return {
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name !== "Button" && name !== "button") return;
        if (!isPerItemRender(node)) return;
        const { text, iconOnly } = accessibleName(node);
        if (!ROW_ACTION_LABEL_RE.test(text)) return;
        // Icon-only „… entfernen“ = chip remover of a form value list.
        if (iconOnly && CHIP_REMOVE_RE.test(text)) return;
        seen += 1;
        if (seen <= tolerated) return;
        context.report({ node, messageId: "rowAction", data: { label: text } });
      },
    };
  },
};

export default {
  meta: { name: "bauart" },
  rules: {
    "one-delete-confirm": oneDeleteConfirm,
    "no-row-action-buttons": noRowActionButtons,
  },
};
