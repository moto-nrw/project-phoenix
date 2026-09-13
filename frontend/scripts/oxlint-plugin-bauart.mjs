// Bauarten ratchet (BAUARTEN-SPEC.md, Abschnitt „Ratschen“).
//
// Five rules; a production match beyond the tolerated remainder fails
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
//                               Shrink-only location baseline
//                               (ROW_ACTION_BASELINE) for the remainder the
//                               issue defers to follow-up PRs. Scope is the
//                               tenant portal: operator, parents and school
//                               portal files are exempt (the spec does not
//                               cover them).
//
//   bauart/no-unconfirmed-destructive-click — a button or menu item whose
//                               label removes something (löschen, entfernen,
//                               archivieren, zurücknehmen, widerrufen,
//                               stornieren) never runs the removal out of
//                               the click itself (Bauart 2 Regel 6, issue
//                               #3109). The tell-tale is an async call fired
//                               straight from the handler: `onClick={() =>
//                               void archive(x)}` or `async () => { await
//                               remove(x) }`. A click that merely opens the
//                               confirmation (`setDeleteTarget(x)`) is
//                               synchronous and passes. The rule reads the
//                               handler inline, so a handler referenced by
//                               name (`onClick={handleDelete}`) is out of
//                               reach — the review covers that case.
//
//   bauart/no-toast-form-error — a form reports its validation and save
//                               errors in the Alert at the top of the edit
//                               area (`error` on FormModal / SlideOverBody,
//                               `FormErrorAlert`), never in a toast (Bauart
//                               2 Regel 5, issue #3113). The rule flags
//                               `toast.error` / `toast.warning` (and the
//                               aliases `toastError` / `toastWarning`)
//                               inside a submit handler: a function named
//                               handleSave / handleSubmit / onSubmit /
//                               save… / submit…, one with a `FormEvent`
//                               parameter, or one that calls
//                               `preventDefault()` itself; callbacks inside
//                               it (`.catch(() => toast.error(…))`) count.
//                               Success toasts and toasts outside such
//                               handlers (delete, toggle, load) pass.
//                               Hard-zero; tenant portal only.
//
//   bauart/no-manage-surface-in-overlay — the Stammdaten of a school are
//                               managed on a route of the Datenverwaltung,
//                               never in a SlideOver, Modal, Popover or
//                               inside a select (Bauart 1 Regel 9, issue
//                               #3114). The rule flags a per-row kebab entry
//                               or button whose label is an object action
//                               (bearbeiten, löschen, archivieren,
//                               wiederherstellen, umbenennen, duplizieren,
//                               (de)aktivieren) rendered inside one of those
//                               shells — directly, through a helper the
//                               shell renders, or in a menu slot of
//                               `ListboxDropdown`. „entfernen“ is not in the
//                               list: inside a dialog it removes a form
//                               value far more often than a stored object,
//                               and no-row-action-buttons covers that case.
//                               Hard-zero; the pinned exceptions
//                               (OWN_OBJECT_ENTRY_EXCEPTIONS) are entries of
//                               the very object the dialog edits, which
//                               Bauart 2 Regel 4 allows.
//
// Files under src/components/ui/ are exempt (ConfirmDeleteModal is itself
// built on Modal and owns the final destructive button), as are tests and
// stories.

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

// Shrink-only per-match tolerance. Each key identifies the exact location of
// a known remainder, so moving or replacing it cannot preserve a file's
// tolerance accidentally. Keys are repo-relative posix paths under src/.
const ROW_ACTION_BASELINE = new Map(
  Object.entries({
    // Noch umzuziehen (#3111, Folge-PRs): Objektaktionen je Zeile.
    "src/app/[tenant]/(protected)/database/students/class-list/page.tsx": [
      "Bearbeiten@466",
      "Löschen@476",
    ],
    "src/app/[tenant]/(protected)/database/students/ended-care/page.tsx": [
      "Endgültig löschen@157",
    ],
    "src/components/admin/pending-invitations-list.tsx": ["Löschen@251"],
    "src/components/database/grade-transitions/grade-transitions-manager.tsx": [
      "Bearbeiten@416",
      "Löschen@436",
    ],
    "src/components/guardians/guardian-list.tsx": ["Bearbeiten@165"],
    "src/components/planning/calendar-periods-editor.tsx": ["Bearbeiten@225"],
    "src/components/settings/trusted-devices-section.tsx": ["Entfernen@158"],
    "src/components/staff/staff-session-table.tsx": [
      "Eintrag nachtragen Block nachtragen Eintrag bearbeiten@850",
      "Block bearbeiten@960",
    ],
    "src/components/staff/stundenkonto-panel.tsx": ["Buchung vom löschen@188"],
    "src/components/students/class-arrival-exception-panel.tsx": [
      "Entfernen@529",
    ],
    "src/components/teachers/caregiver-blocker-resolution-panel.tsx": [
      "Übertragen Entfernen@566",
      "Übertragen Entfernen@630",
    ],
    "src/components/timetable/period-switcher-dropdown.tsx": ["bearbeiten@310"],
    // Formular-intern (Eintrag eines Formularwerts, kein gespeichertes
    // Objekt): fest an die bestehende Stelle gebunden, damit keine neue
    // Zeilenaktion dieselbe Ausnahme nutzen kann.
    "src/app/[tenant]/(protected)/calendar/page.tsx": ["entfernen@1190"],
    "src/app/[tenant]/(protected)/meal-plan/page.tsx": [
      "Gericht entfernen@738",
    ],
    "src/app/[tenant]/(protected)/parent-announcements/page.tsx": [
      "Antwort entfernen@1193",
      "Entfernen@1465",
    ],
    "src/components/enrollment/care-offerings-editor.tsx": [
      "Bedingung löschen@1974",
    ],
    "src/components/enrollment/enrollment-form-editor.tsx": [
      "abweichend bearbeiten@2512",
      "Auswahlzeit entfernen@3436",
    ],
    "src/components/guardians/guardian-form-modal.tsx": [
      "Entfernen@585",
      "Telefonnummer entfernen@844",
    ],
    "src/components/staff/shift-edit-modal.tsx": ["Entfernen@1338"],
    "src/components/staff/stammdaten-section-forms.tsx": [
      "Qualifikation entfernen@415",
    ],
    "src/components/students/companion-picker.tsx": ["entfernen@274"],
    "src/components/students/student-create-modal.tsx": [
      "Erziehungsberechtigte/n entfernen@691",
    ],
    "src/components/timetable/substitution-slide-over.tsx": [
      "Rückgängig Entfernen@765",
    ],
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
 * aria-label, then text content, then title. */
function accessibleName(openingElement) {
  const chunks = [];
  collectChildText(openingElement.parent, chunks);
  const childText = chunks.join(" ").replaceAll(/\s+/g, " ").trim();
  const aria = staticText(jsxAttribute(openingElement, "aria-label")?.value);
  if (aria) return { text: aria };
  if (childText) return { text: childText };
  return { text: staticText(jsxAttribute(openingElement, "title")?.value) };
}

/** A form-value remover is an inline chip with its conventional X icon.
 * A surrounding form or a field elsewhere in a mapped row does not establish
 * that this button removes that local value: persisted object rows can have
 * both, and must still use the kebab. */
function isFormValueRemover(openingElement) {
  const button = openingElement.parent;
  const hasXIcon = button?.children?.some(
    (child) =>
      child.type === "JSXElement" && jsxName(child.openingElement.name) === "X",
  );

  let current = button?.parent;
  while (current) {
    if (current.type === "JSXElement") {
      const className = staticText(
        jsxAttribute(current.openingElement, "className")?.value,
      );
      if (
        hasXIcon &&
        /\binline-flex\b/.test(className) &&
        /\brounded-/.test(className)
      ) {
        return true;
      }
    }
    current = current.parent;
  }
  return false;
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
    const tolerated = new Set(ROW_ACTION_BASELINE.get(key) ?? []);

    return {
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name !== "Button" && name !== "button") return;
        if (!isPerItemRender(node)) return;
        const { text } = accessibleName(node);
        if (!ROW_ACTION_LABEL_RE.test(text)) return;
        // Form values may be removed inline; stored objects use the kebab.
        if (CHIP_REMOVE_RE.test(text) && isFormValueRemover(node)) {
          return;
        }
        if (tolerated.delete(`${text}@${node.loc.start.line}`)) return;
        context.report({ node, messageId: "rowAction", data: { label: text } });
      },
    };
  },
};

// --- bauart/no-unconfirmed-destructive-click ---------------------------------

// Labels that announce a removal. Word-bounded on unicode letters like
// DELETE_LABEL_RE. „Zurücksetzen“ is deliberately absent: it names form and
// filter resets far more often than a removal.
const DESTRUCTIVE_LABEL_RE =
  /(?:^|\P{L})(?:löschen|entfernen|archivieren|zurücknehmen|widerrufen|stornieren)(?:\P{L}|$)/iu;
// next-intl calls such as t("remove") carry the locale key, not visible
// text, in the syntax tree. Treat established destructive key segments as
// labels so localized portals stay behind the same confirmation ratchet.
const DESTRUCTIVE_TRANSLATION_KEY_RE =
  /(?:^|[._-])(?:delete|remove|archive|revoke|withdraw|cancel)(?=$|[._-]|[A-Z])/i;

const CLICKABLE_ELEMENTS = new Set(["button", "Button"]);

/** Text a person reads on the element: aria-label / title attributes and
 *  every static string inside its children. */
function collectLabelStrings(openingElement, parentElement) {
  const chunks = [];
  for (const name of ["aria-label", "title"]) {
    const attribute = jsxAttribute(openingElement, name);
    if (attribute?.value) collectStaticStrings(attribute.value, chunks);
  }
  for (const child of parentElement?.children ?? []) {
    if (child.type === "JSXText") {
      chunks.push(child.value);
    } else if (child.type === "JSXExpressionContainer") {
      collectStaticStrings(child.expression, chunks);
    } else if (child.type === "JSXElement") {
      chunks.push(...collectLabelStrings(child.openingElement, child));
    } else if (child.type === "JSXFragment") {
      for (const nested of child.children) {
        if (nested.type === "JSXText") chunks.push(nested.value);
        else if (nested.type === "JSXExpressionContainer")
          collectStaticStrings(nested.expression, chunks);
        else if (nested.type === "JSXElement")
          chunks.push(...collectLabelStrings(nested.openingElement, nested));
      }
    }
  }
  return chunks;
}

function mentionsDestruction(chunks) {
  return chunks.some(
    (chunk) =>
      DESTRUCTIVE_LABEL_RE.test(chunk) ||
      DESTRUCTIVE_TRANSLATION_KEY_RE.test(chunk),
  );
}

function isCall(node) {
  return node?.type === "CallExpression";
}

function calledFunctionName(call) {
  if (call.callee?.type === "Identifier") return call.callee.name;
  if (
    call.callee?.type === "MemberExpression" &&
    call.callee.property?.type === "Identifier"
  ) {
    return call.callee.property.name;
  }
  return null;
}

/** State setters and conventionally named confirmation openers are the one
 *  direct-call form allowed in a destructive click: they only show the
 *  confirmation, while its onConfirm owns the action. */
function opensConfirmation(call) {
  const name = calledFunctionName(call);
  return (
    name !== null &&
    /^(?:set|open|show|begin|request|onRequest)[A-Z]/.test(name)
  );
}

function isDelegatedHandlerCall(call) {
  const name = calledFunctionName(call);
  return name !== null && /^on[A-Z]/.test(name);
}

/** `void call(...)`, `await call(...)`, a directly returned call, or a
 *  `.then(...)`/`.catch(...)` chain on a call: the handler itself runs an
 *  action. We have no type information here: delegated `on…` handlers stay
 *  outside this syntactic ratchet, while direct calls with irreversible verbs
 *  are guarded unless they only open the confirmation. */
function isFiredAsyncCall(expression) {
  if (!expression) return false;
  if (expression.type === "UnaryExpression" && expression.operator === "void") {
    return (
      isCall(expression.argument) && !opensConfirmation(expression.argument)
    );
  }
  if (expression.type === "AwaitExpression") {
    return (
      isCall(expression.argument) && !opensConfirmation(expression.argument)
    );
  }
  if (
    isCall(expression) &&
    expression.callee?.type === "MemberExpression" &&
    expression.callee.property?.type === "Identifier" &&
    (expression.callee.property.name === "then" ||
      expression.callee.property.name === "catch")
  ) {
    return true;
  }
  if (isCall(expression)) {
    const name = calledFunctionName(expression);
    return (
      !opensConfirmation(expression) &&
      !isDelegatedHandlerCall(expression) &&
      name !== null &&
      (/^(?:delete|remove)$/i.test(name) ||
        /^(?:archive|revoke|withdraw|cancel)[A-Z]/.test(name))
    );
  }
  return false;
}

/** Any fired async call anywhere in the handler body, branches included
 *  (`if (x) void remove()` is still the click running the removal). Nested
 *  functions are skipped: a callback handed to something else is not the
 *  click. */
function containsFiredAsyncCall(node, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return false;
  seen.add(node);
  if (isFiredAsyncCall(node)) return true;
  if (
    node.type === "ArrowFunctionExpression" ||
    node.type === "FunctionExpression"
  ) {
    return false;
  }
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      if (value.some((child) => containsFiredAsyncCall(child, seen)))
        return true;
    } else if (containsFiredAsyncCall(value, seen)) {
      return true;
    }
  }
  return false;
}

/** Does the inline handler fire the action itself? */
function handlerFiresAction(handler) {
  if (
    handler?.type !== "ArrowFunctionExpression" &&
    handler?.type !== "FunctionExpression"
  ) {
    return false;
  }
  return containsFiredAsyncCall(handler.body);
}

const noUnconfirmedDestructiveClick = {
  meta: {
    type: "problem",
    docs: {
      description:
        "A button or menu item that removes something opens the confirmation; it never runs the removal out of the click.",
    },
    messages: {
      element:
        "„{{label}}“ läuft direkt aus dem Klick. Der Klick öffnet die Rückfrage (ConfirmDeleteModal, für Zustandswechsel ConfirmationModal); die Aktion läuft in deren onConfirm (BAUARTEN-SPEC Bauart 2 Regel 6, #3109).",
      menuItem:
        "Menüeintrag „{{label}}“ läuft direkt aus dem Klick. Der Eintrag öffnet die Rückfrage (ConfirmDeleteModal, für Zustandswechsel ConfirmationModal); die Aktion läuft in deren onConfirm (BAUARTEN-SPEC Bauart 2 Regel 6, #3109).",
    },
    schema: [],
  },
  create(context) {
    if (isExempt(context)) return {};

    const firstMatch = (chunks) =>
      chunks.find((chunk) => DESTRUCTIVE_LABEL_RE.test(chunk))?.trim();

    return {
      JSXElement(node) {
        const opening = node.openingElement;
        const name = jsxName(opening.name);
        if (!CLICKABLE_ELEMENTS.has(name)) return;
        const onClick = jsxAttribute(opening, "onClick");
        if (onClick?.value?.type !== "JSXExpressionContainer") return;
        if (!handlerFiresAction(onClick.value.expression)) return;
        const chunks = collectLabelStrings(opening, node);
        if (!mentionsDestruction(chunks)) return;
        context.report({
          node: opening,
          messageId: "element",
          data: { label: firstMatch(chunks) },
        });
      },
      ObjectExpression(node) {
        let label = null;
        let onClick = null;
        for (const property of node.properties) {
          if (property.type !== "Property" || property.computed) continue;
          const key =
            property.key.type === "Identifier"
              ? property.key.name
              : property.key.type === "Literal"
                ? String(property.key.value)
                : null;
          if (key === "label") label = property.value;
          if (key === "onClick") onClick = property.value;
        }
        if (!label || !onClick) return;
        if (!handlerFiresAction(onClick)) return;
        const chunks = [];
        collectStaticStrings(label, chunks);
        if (!mentionsDestruction(chunks)) return;
        context.report({
          node,
          messageId: "menuItem",
          data: { label: firstMatch(chunks) },
        });
      },
    };
  },
};

// ---------------------------------------------------------------------------
// bauart/no-autosave — automatic saving exists only where it is named
// (Bauart 4, Bauart 2 Regel 4, issue #3112). A form control that persists on
// blur or on change, without a „Speichern“ below it, is the pattern: the
// inline handler fires an async call (`void save(x)`, `await update(x)`, a
// direct Promise-returning call, `.then(`) straight out of `onBlur`, or out
// of a change handler on a kit
// control when the callee reads like a write (save, update, persist, patch,
// set…, submit, store, assign, link, mutate). Reads out of a change handler
// (`void search(q)`) pass; the search field of a list is not a save.
//
// Exempt: the settings page (the one Bauart that auto-saves, and says so), the
// portals outside the spec (operator), the UI kit itself, tests and stories.
// The remaining named files are the shrink-only baseline: flächen that name
// their auto-save on screen and are documented in issue #3112. New entries
// need the same sentence on the surface and a reviewer's approval.

const AUTOSAVE_EXEMPT_DIR_RE =
  /(?:^|\/)src\/(?:components\/settings\/|components\/operator\/|app\/operator\/)/;

const AUTOSAVE_BASELINE = new Set([
  // Bauart 4 by decision (#3112): Lohnarten are configuration. Both cards
  // carry „Änderungen werden sofort gespeichert.“
  "src/app/[tenant]/(protected)/payroll/page.tsx",
  // Parents portal (outside the spec); names it as „Änderungen werden
  // automatisch gespeichert.“ and guards unsaved input (#3112).
  "src/components/parent/child-master-data.tsx",
  // Language switch: applies immediately by nature, persisted as a courtesy.
  "src/components/parent/language-switcher.tsx",
]);

const CHANGE_HANDLER_ATTRIBUTES = new Set([
  "onChange",
  "onValueChange",
  "onCheckedChange",
  "onSelect",
]);

const FORM_CONTROL_ELEMENTS = new Set([
  "input",
  "select",
  "textarea",
  "Input",
  "Textarea",
  "Checkbox",
  "Radio",
  "CustomSelect",
  "ListboxDropdown",
  "SegmentedControl",
  "MultiSelect",
  "MultiCheckboxSelect",
  "DatePicker",
  "TimeField",
  "ToggleChip",
]);

// `toggle…`/`change…` stay out on purpose: they name UI state and previews far
// more often than writes (a `toggleExcluded` that only re-runs a preview).
// The header comment above still lists them as examples of the shape; the
// regex is the contract.
const WRITE_VERB_RE =
  /^(?:save|update|persist|patch|put|submit|store|assign|link|unlink|mutate|write|set[A-Z]|create|remove|delete)/;

function relativeFileName(context) {
  const name = fileName(context);
  const index = name.indexOf("/src/");
  return index === -1 ? name : name.slice(index + 1);
}

function isAutosaveExempt(context) {
  if (isExempt(context)) return true;
  const name = fileName(context);
  if (AUTOSAVE_EXEMPT_DIR_RE.test(name)) return true;
  return AUTOSAVE_BASELINE.has(relativeFileName(context));
}

/** `void call()`, `await call()`, a direct call, or a `.then(`/`.catch(` chain
 *  from the handler. Returns the fired call and whether it is direct. */
function firedCall(expression) {
  if (!expression) return null;
  if (
    (expression.type === "UnaryExpression" && expression.operator === "void") ||
    expression.type === "AwaitExpression"
  ) {
    return isCall(expression.argument)
      ? { call: expression.argument, direct: false }
      : null;
  }
  if (
    isCall(expression) &&
    expression.callee?.type === "MemberExpression" &&
    expression.callee.property?.type === "Identifier" &&
    (expression.callee.property.name === "then" ||
      expression.callee.property.name === "catch")
  ) {
    let inner = expression.callee.object;
    while (
      isCall(inner) &&
      inner.callee?.type === "MemberExpression" &&
      inner.callee.property?.type === "Identifier" &&
      (inner.callee.property.name === "then" ||
        inner.callee.property.name === "catch")
    ) {
      inner = inner.callee.object;
    }
    return { call: isCall(inner) ? inner : expression, direct: false };
  }
  if (isCall(expression)) return { call: expression, direct: true };
  return null;
}

/** First fired call anywhere in the handler body (nested functions skipped,
 *  like containsFiredAsyncCall) that satisfies `accept`. */
function findFiredCall(node, accept, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return null;
  seen.add(node);
  const fired = firedCall(node);
  if (fired && accept(fired.call, fired.direct)) return fired.call;
  if (
    node.type === "ArrowFunctionExpression" ||
    node.type === "FunctionExpression"
  ) {
    return null;
  }
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      for (const child of value) {
        const found = findFiredCall(child, accept, seen);
        if (found) return found;
      }
    } else {
      const found = findFiredCall(value, accept, seen);
      if (found) return found;
    }
  }
  return null;
}

function inlineHandlerBody(attribute) {
  if (attribute?.value?.type !== "JSXExpressionContainer") return null;
  const handler = attribute.value.expression;
  if (
    handler?.type !== "ArrowFunctionExpression" &&
    handler?.type !== "FunctionExpression"
  ) {
    return null;
  }
  return handler.body;
}

function isWriteCall(call, reactStateSetters) {
  const name = calledFunctionName(call);
  return (
    name !== null && !reactStateSetters.has(name) && WRITE_VERB_RE.test(name)
  );
}

function isPromiseReturningFunctionType(annotation) {
  const type = annotation?.typeAnnotation;
  return (
    type?.type === "TSFunctionType" &&
    type.returnType?.typeAnnotation?.type === "TSTypeReference" &&
    type.returnType.typeAnnotation.typeName?.type === "Identifier" &&
    type.returnType.typeAnnotation.typeName.name === "Promise"
  );
}

function addPromiseReturningParameterNames(params, asyncFunctions) {
  for (const parameter of params ?? []) {
    if (
      parameter?.type === "Identifier" &&
      isPromiseReturningFunctionType(parameter.typeAnnotation)
    ) {
      asyncFunctions.add(parameter.name);
      continue;
    }
    const members = parameter?.typeAnnotation?.typeAnnotation?.members;
    if (!Array.isArray(members)) continue;
    for (const member of members) {
      if (
        member?.type === "TSPropertySignature" &&
        member.key?.type === "Identifier" &&
        isPromiseReturningFunctionType(member.typeAnnotation)
      ) {
        asyncFunctions.add(member.key.name);
      }
    }
  }
}

const noAutosave = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Automatisches Speichern gibt es nur in den Einstellungen (Bauart 4): kein Schreiben aus onBlur oder aus dem Change-Handler eines Formularfelds ohne „Speichern“ darunter (#3112).",
    },
    messages: {
      blur: "onBlur schreibt sofort ({{ call }}). Bearbeiten-Zustand mit „Speichern“ unten (ui/EditActions) statt Auto-Save; Auto-Save nur in den Einstellungen und dort benannt (BAUARTEN-SPEC, Bauart 2 Regel 4, #3112).",
      change:
        "{{ element }} speichert im {{ attribute }} sofort ({{ call }}). Entwurf im Bearbeiten-Zustand halten und mit „Speichern“ schreiben; Auto-Save nur in den Einstellungen und dort benannt (BAUARTEN-SPEC, Bauart 2 Regel 4, #3112).",
    },
  },
  create(context) {
    if (isAutosaveExempt(context)) return {};
    const reactStateSetters = new Set();
    const asyncFunctions = new Set();
    const importedFunctions = new Set();

    return {
      ImportDeclaration(node) {
        if (node.importKind === "type") return;
        for (const specifier of node.specifiers ?? []) {
          if (
            specifier.type !== "ImportNamespaceSpecifier" &&
            specifier.importKind !== "type" &&
            specifier.local?.type === "Identifier"
          ) {
            importedFunctions.add(specifier.local.name);
          }
        }
      },
      FunctionDeclaration(node) {
        if (node.async && node.id?.type === "Identifier") {
          asyncFunctions.add(node.id.name);
        }
        addPromiseReturningParameterNames(node.params, asyncFunctions);
      },
      VariableDeclarator(node) {
        const setter = node.id?.elements?.[1];
        if (
          setter?.type === "Identifier" &&
          isCall(node.init) &&
          calledFunctionName(node.init) === "useState"
        ) {
          reactStateSetters.add(setter.name);
        }
        if (
          node.init?.type === "ArrowFunctionExpression" ||
          node.init?.type === "FunctionExpression"
        ) {
          addPromiseReturningParameterNames(node.init.params, asyncFunctions);
          if (node.id?.type === "Identifier" && node.init.async) {
            asyncFunctions.add(node.id.name);
          }
        }
      },
      JSXElement(node) {
        const opening = node.openingElement;
        const element = jsxName(opening.name);

        const onBlur = jsxAttribute(opening, "onBlur");
        const blurBody = inlineHandlerBody(onBlur);
        if (blurBody) {
          const call = findFiredCall(
            blurBody,
            (fired, direct) =>
              isWriteCall(fired, reactStateSetters) &&
              (!direct ||
                asyncFunctions.has(calledFunctionName(fired)) ||
                importedFunctions.has(calledFunctionName(fired))),
          );
          if (call) {
            context.report({
              node: onBlur,
              messageId: "blur",
              data: { call: calledFunctionName(call) ?? "…" },
            });
          }
        }

        if (!FORM_CONTROL_ELEMENTS.has(element)) return;
        for (const attributeName of CHANGE_HANDLER_ATTRIBUTES) {
          const attribute = jsxAttribute(opening, attributeName);
          const body = inlineHandlerBody(attribute);
          if (!body) continue;
          const call = findFiredCall(
            body,
            (fired, direct) =>
              isWriteCall(fired, reactStateSetters) &&
              (!direct ||
                asyncFunctions.has(calledFunctionName(fired)) ||
                importedFunctions.has(calledFunctionName(fired))),
          );
          if (!call) continue;
          context.report({
            node: attribute,
            messageId: "change",
            data: {
              element,
              attribute: attributeName,
              call: calledFunctionName(call) ?? "…",
            },
          });
        }
      },
    };
  },
};

// --- bauart/no-toast-form-error ----------------------------------------------

// The toast API of ~/contexts/ToastContext (`toast.error`, `toast.warning`),
// including the destructured aliases the code base uses
// (`const { error: toastError } = useToast()`).
const TOAST_LEVEL_RE = /^(?:error|warning|warn)$/;
const TOAST_ALIAS_RE = /^toast(?:Error|Warning|Warn)$/;
// A function that saves what a person typed. Matches handleSave, handleSubmit,
// onSubmit, submitForm, saveDraft, handleSaveClick; not handleSelect or
// handleToggle, whose toast is not a form's error report.
const SUBMIT_HANDLER_NAME_RE = /^(?:handle|on)?(?:submit|save)/i;

function isToastErrorCall(node) {
  if (!isCall(node)) return false;
  const { callee } = node;
  if (
    callee?.type === "MemberExpression" &&
    callee.property?.type === "Identifier" &&
    TOAST_LEVEL_RE.test(callee.property.name)
  ) {
    return (
      callee.object?.type === "Identifier" && /toast/i.test(callee.object.name)
    );
  }
  return callee?.type === "Identifier" && TOAST_ALIAS_RE.test(callee.name);
}

function isFunctionNode(node) {
  return (
    node?.type === "ArrowFunctionExpression" ||
    node?.type === "FunctionExpression" ||
    node?.type === "FunctionDeclaration"
  );
}

/** `event.preventDefault()` in the function's own body (nested functions
 *  skipped): the handler intercepts a form submission. */
function callsPreventDefault(node, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return false;
  seen.add(node);
  if (
    isCall(node) &&
    node.callee?.type === "MemberExpression" &&
    node.callee.property?.type === "Identifier" &&
    node.callee.property.name === "preventDefault"
  ) {
    return true;
  }
  if (isFunctionNode(node)) return false;
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      if (value.some((child) => callsPreventDefault(child, seen))) return true;
    } else if (callsPreventDefault(value, seen)) {
      return true;
    }
  }
  return false;
}

/** The name a function is bound to: `const handleSave = async () => {}`,
 *  `const handleSave = useCallback(async () => {}, [])`,
 *  `function handleSave() {}`, `{ onSubmit: () => {} }`. */
function boundFunctionName(fn) {
  if (fn.type === "FunctionDeclaration") return fn.id?.name ?? null;
  let owner = fn.parent;
  // useCallback(fn, deps) and similar wrappers sit between the arrow and its
  // declarator.
  if (isCall(owner) && owner.arguments.includes(fn)) owner = owner.parent;
  if (owner?.type === "VariableDeclarator" && owner.id?.type === "Identifier") {
    return owner.id.name;
  }
  if (owner?.type === "Property" && owner.key?.type === "Identifier") {
    return owner.key.name;
  }
  if (owner?.type === "JSXExpressionContainer") {
    const attribute = owner.parent;
    if (
      attribute?.type === "JSXAttribute" &&
      attribute.name?.type === "JSXIdentifier"
    ) {
      return attribute.name.name;
    }
  }
  return null;
}

function hasFormEventParameter(fn) {
  return (fn.params ?? []).some((param) => {
    const annotation = param.typeAnnotation?.typeAnnotation;
    const chunks = [];
    collectTypeNames(annotation, chunks);
    return chunks.some((name) => /FormEvent$/.test(name));
  });
}

function collectTypeNames(node, chunks, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return;
  seen.add(node);
  if (node.type === "Identifier" && typeof node.name === "string") {
    chunks.push(node.name);
  }
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      for (const child of value) collectTypeNames(child, chunks, seen);
    } else {
      collectTypeNames(value, chunks, seen);
    }
  }
}

function isSubmitHandler(fn) {
  const name = boundFunctionName(fn);
  if (name && SUBMIT_HANDLER_NAME_RE.test(name)) return true;
  if (hasFormEventParameter(fn)) return true;
  return callsPreventDefault(fn.body);
}

/** The nearest enclosing submit handler, looking through callbacks such as
 *  `.catch((err) => toast.error(…))` inside it. */
function enclosingSubmitHandler(node) {
  let current = node.parent;
  while (current) {
    if (isFunctionNode(current) && isSubmitHandler(current)) return current;
    current = current.parent;
  }
  return null;
}

const noToastFormError = {
  meta: {
    type: "problem",
    docs: {
      description:
        "A form reports its validation and save errors in an Alert at the top of the edit area (and at the field), not in a toast.",
    },
    messages: {
      toast:
        "Fehler aus dem Speichern eines Formulars („{{handler}}“) stehen im Alert oben im Bearbeiten-Bereich und, wo zuordenbar, am Feld: `error` an FormModal oder SlideOverBody, `error` am Input. Ein Toast verblasst, bevor jemand das Feld gefunden hat (BAUARTEN-SPEC Bauart 2 Regel 5, #3113).",
    },
    schema: [],
  },
  create(context) {
    if (isExempt(context)) return {};
    if (OTHER_PORTAL_RE.test(fileKey(context))) return {};

    return {
      CallExpression(node) {
        if (!isToastErrorCall(node)) return;
        const handler = enclosingSubmitHandler(node);
        if (!handler) return;
        context.report({
          node,
          messageId: "toast",
          data: { handler: boundFunctionName(handler) ?? "onSubmit" },
        });
      },
    };
  },
};

// --- bauart/no-manage-surface-in-overlay --------------------------------------

// Shells that float over a page. Anything rendered inside one of them is an
// overlay; a kebab per list row in there is a management surface in a dialog.
const OVERLAY_ELEMENTS = new Set([
  "SlideOver",
  "SlideOverContent",
  "SlideOverBody",
  "Modal",
  "FormModal",
  "ChoiceModal",
  "ConfirmationModal",
  "ConfirmDeleteModal",
  "Drawer",
  "DrawerContent",
  "AnchoredPopover",
  "Popover",
  "PopoverContent",
  "ListboxDropdown",
]);
// Slots of the kit listbox that render inside its floating menu. A button
// per option there manages the option list from inside a select.
const LISTBOX_MENU_SLOTS = new Set([
  "renderOptionActions",
  "menuHeader",
  "menuFooter",
]);
// Object actions that make a row a managed object. „entfernen“ is absent on
// purpose: inside a dialog it removes a form value (a staged guardian, a
// chip) far more often than a stored object, and that case has its own rule.
const MANAGE_ACTION_LABEL_RE =
  /(?:^|\P{L})(?:bearbeiten|löschen|archivieren|wiederherstellen|umbenennen|duplizieren|deaktivieren|aktivieren)(?:\P{L}|$)/iu;

// Einträge des Objekts, um das der Dialog ohnehin geht — keine Stammdaten,
// die anderswo in einer Auswahl stehen. Bauart 2 Regel 4 lässt das Nachtragen
// eines Eintrags am Objekt ausdrücklich zu; verboten ist der Katalog der
// Schule in einem Overlay. Ortsgebunden, damit die Ausnahme nicht wandert.
const OWN_OBJECT_ENTRY_EXCEPTIONS = new Map(
  Object.entries({
    // Teilentschuldigungen genau dieses Kindes an genau diesen Tagen.
    "src/components/students/planned-status-days-modal.tsx": [
      "Bearbeiten@1142",
    ],
  }),
);

/** Static labels of the entries in an OverflowMenu `items` expression. */
function collectMenuItemLabels(node, labels, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return;
  seen.add(node);
  if (
    node.type === "Property" &&
    node.key?.type === "Identifier" &&
    node.key.name === "label"
  ) {
    const text = staticText(node.value);
    if (text) labels.push(text);
    return;
  }
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      for (const child of value) collectMenuItemLabels(child, labels, seen);
    } else {
      collectMenuItemLabels(value, labels, seen);
    }
  }
}

/** Ein Menü-Slot der Kit-Auswahlliste, als Objekteintrag (der Hook baut die
 *  Props) oder als JSX-Attribut (die Komponente bekommt sie direkt). */
function isListboxMenuSlot(node) {
  if (
    node?.type === "Property" &&
    node.key?.type === "Identifier" &&
    LISTBOX_MENU_SLOTS.has(node.key.name)
  ) {
    return true;
  }
  return (
    node?.type === "JSXAttribute" &&
    node.name?.type === "JSXIdentifier" &&
    LISTBOX_MENU_SLOTS.has(node.name.name)
  );
}

/**
 * Where a node sits: `overlay` when a JSX ancestor is an overlay shell or a
 * listbox menu slot, `inJsx` when it is rendered at all, `binding` = the
 * nearest `const x = …` / `function x()` around it. The binding is what
 * links a helper such as `renderManageView` or `listFooter` to the overlay
 * that renders it.
 */
function locate(node) {
  let current = node.parent;
  let overlay = false;
  let inJsx = false;
  let inListboxSlot = false;
  let binding = null;
  while (current) {
    if (current.type === "JSXExpressionContainer") inJsx = true;
    if (
      current.type === "JSXElement" &&
      OVERLAY_ELEMENTS.has(jsxName(current.openingElement.name))
    ) {
      overlay = true;
    }
    if (isListboxMenuSlot(current)) {
      overlay = true;
      inListboxSlot = true;
    }
    if (binding === null) {
      if (
        current.type === "VariableDeclarator" &&
        current.id?.type === "Identifier"
      ) {
        binding = current.id.name;
      } else if (current.type === "FunctionDeclaration" && current.id) {
        binding = current.id.name;
      }
    }
    current = current.parent;
  }
  return { overlay, inJsx, inListboxSlot, binding };
}

const noManageSurfaceInOverlay = {
  meta: {
    type: "problem",
    docs: {
      description:
        "A collection with per-object actions (Bearbeiten, Archivieren, Löschen, …) is a route in the Datenverwaltung, never a view inside a SlideOver, Modal, Popover or select.",
    },
    messages: {
      manageSurface:
        "Verwaltungsfläche im Overlay: die Zeilenaktion „{{label}}“ liegt in einem {{shell}}. Sammlung und Objektbearbeitung sind eine Route der Datenverwaltung (Bauart 1 + 2); ein Formular, dem ein Stammdatum fehlt, trägt nur die Auswahl und einen Link „… verwalten“ zur Route (BAUARTEN-SPEC Bauart 1 Regel 9, #3114).",
    },
    schema: [],
  },
  create(context) {
    if (isExempt(context)) return {};
    if (OTHER_PORTAL_RE.test(fileKey(context))) return {};
    const ownEntries = new Set(
      OWN_OBJECT_ENTRY_EXCEPTIONS.get(fileKey(context)) ?? [],
    );

    // Names rendered directly inside an overlay, and the names each binding
    // renders in turn; the closure of the first over the second is every
    // helper whose JSX ends up in an overlay.
    const overlayReferences = new Set();
    const referencesByBinding = new Map();
    const pending = [];

    const record = (node, label) => {
      if (ownEntries.delete(`${label}@${node.loc.start.line}`)) return;
      const { overlay, inListboxSlot, binding } = locate(node);
      const shell = inListboxSlot ? "Auswahlfeld" : "Overlay";
      if (overlay) {
        context.report({
          node,
          messageId: "manageSurface",
          data: { label, shell },
        });
        return;
      }
      if (binding) pending.push({ node, label, binding });
    };

    return {
      Identifier(node) {
        const parent = node.parent;
        // Only expression references count: not the name being declared,
        // not a property key, not a parameter.
        if (
          (parent?.type === "VariableDeclarator" && parent.id === node) ||
          (parent?.type === "Property" && parent.key === node) ||
          (parent?.type === "MemberExpression" && parent.property === node) ||
          parent?.type === "FunctionDeclaration"
        ) {
          return;
        }
        const { overlay, inJsx, binding } = locate(node);
        if (!inJsx) return;
        if (overlay) {
          overlayReferences.add(node.name);
          return;
        }
        if (!binding || binding === node.name) return;
        let references = referencesByBinding.get(binding);
        if (!references) {
          references = new Set();
          referencesByBinding.set(binding, references);
        }
        references.add(node.name);
      },
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name === "OverflowMenu") {
          if (!isPerItemRender(node)) return;
          const labels = [];
          collectMenuItemLabels(jsxAttribute(node, "items")?.value, labels);
          const label = labels.find((text) =>
            MANAGE_ACTION_LABEL_RE.test(text),
          );
          if (label) record(node, label);
          return;
        }
        if (name !== "Button" && name !== "button") return;
        const { inListboxSlot } = locate(node);
        if (!inListboxSlot && !isPerItemRender(node)) return;
        const { text } = accessibleName(node);
        if (MANAGE_ACTION_LABEL_RE.test(text)) record(node, text);
      },
      "Program:exit"() {
        const scoped = new Set(overlayReferences);
        let grew = true;
        while (grew) {
          grew = false;
          for (const name of [...scoped]) {
            for (const reference of referencesByBinding.get(name) ?? []) {
              if (!scoped.has(reference)) {
                scoped.add(reference);
                grew = true;
              }
            }
          }
        }
        for (const { node, label, binding } of pending) {
          if (!scoped.has(binding)) continue;
          context.report({
            node,
            messageId: "manageSurface",
            data: { label, shell: "Overlay" },
          });
        }
      },
    };
  },
};

// ---------------------------------------------------------------------------
// bauart/one-detail-per-type — pro Objekttyp genau eine Objektansicht
// (Bauart 2, Bauart 1 Regel 2, #3115). Drei Erscheinungsformen eines zweiten
// Detailbaums werden erkannt:
//
//   1. `MasterDetailLayout` außerhalb der Typen, deren Pane die einzige
//      Objektansicht im Portal ist. Die Liste ist fest: ein neuer Typ mit
//      Pane braucht eine Entscheidung, ob er eine Route bekommt.
//   2. Die Pane-Bauteile (`DetailPanel`, `DatabaseDetailHeader`) in einem
//      Overlay.
//   3. Die Feldgruppen der Objektansicht (`InfoSection`, `DataGrid`) in
//      einem `SlideOver` oder `Drawer`: ein Slide-over ist nie eine
//      Detailansicht. Zentrierte Modale bleiben außen vor, weil ein
//      `FormModal` ein Feld zur Einordnung zeigen darf.
// ---------------------------------------------------------------------------
const MASTER_DETAIL_LAYOUT_SOURCE_RE = /(?:^|\/)master-detail-layout$/;
// Typen, deren Pane die einzige Objektansicht ist: Gruppe, Gerät, Rolle,
// Aktivität, Berechtigung und die Kataloge der Datenverwaltung (#3114).
const MASTER_DETAIL_ALLOWED_FILES = new Set([
  "src/components/database/master-detail-layout.tsx",
  "src/components/database/master-detail-skeleton.tsx",
  "src/components/database/catalog/catalog-page.tsx",
  "src/components/activities/activities-master-detail.tsx",
  "src/components/devices/devices-master-detail.tsx",
  "src/components/groups/groups-master-detail.tsx",
  "src/components/permissions/permissions-master-detail.tsx",
  "src/components/roles/roles-master-detail.tsx",
]);
const PANE_ELEMENTS = new Set(["DetailPanel", "DatabaseDetailHeader"]);
const DETAIL_VIEW_ELEMENTS = new Set(["InfoSection", "DataGrid"]);
const SLIDE_SHELLS = new Set([
  "SlideOver",
  "SlideOverContent",
  "SlideOverBody",
  "Drawer",
  "DrawerContent",
]);
// Ein Formular im Slide-over darf Felder zur Einordnung zeigen (welche
// Gruppe, wie groß), so wie ein `FormModal`. Ortsgebunden, damit die
// Ausnahme nicht auf eine echte Detailansicht wandert.
const FORM_CONTEXT_EXCEPTIONS = new Map(
  Object.entries({
    // Übergabe einer Gruppe: Erklärung und die Gruppe, um die es geht.
    "src/components/groups/group-transfer-modal.tsx": [
      "InfoSection@232",
      "DataGrid@246",
    ],
  }),
);

/** Der nächste JSX-Vorfahre, dessen Name in `names` steht. */
function enclosingElement(node, names) {
  let current = node.parent;
  while (current) {
    if (
      current.type === "JSXElement" &&
      names.has(jsxName(current.openingElement.name))
    ) {
      return jsxName(current.openingElement.name);
    }
    current = current.parent;
  }
  return null;
}

const oneDetailPerType = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Each object type has exactly one object view (its route or, for types without a route, the pane next to the list). No pane, slide-over or modal as a second detail tree.",
    },
    messages: {
      secondPane:
        "Zweiter Detailbaum: `MasterDetailLayout` ist nur für die Typen erlaubt, deren Pane die einzige Objektansicht ist (feste Liste in scripts/oxlint-plugin-bauart.mjs). Hat der Typ eine Route, verlinkt die Zeile dorthin (`DatabaseListItem href`, `?from=` trägt den Rückweg); sonst gehört der Typ in die Liste (BAUARTEN-SPEC Bauart 1 Regel 2, #3115).",
      paneInOverlay:
        "Zweiter Detailbaum: `{{element}}` liegt in einem Overlay. Die Objektansicht eines Typs ist eine Route oder das Pane neben der Liste, nie ein Slide-over oder Modal (BAUARTEN-SPEC Bauart 2, #3115).",
      detailInSlideOver:
        "Detailansicht im Slide-over: `{{element}}` liegt in `{{shell}}`. Ein Slide-over ist nie eine Objektansicht; die Felder gehören auf die Route des Typs (BAUARTEN-SPEC Bauart 1 Regel 2, #3115).",
    },
    schema: [],
  },
  create(context) {
    if (isExempt(context)) return {};
    if (OTHER_PORTAL_RE.test(fileKey(context))) return {};
    const file = fileKey(context);
    const formContext = new Set(FORM_CONTEXT_EXCEPTIONS.get(file) ?? []);

    return {
      ImportDeclaration(node) {
        const source = String(node.source?.value ?? "");
        if (!MASTER_DETAIL_LAYOUT_SOURCE_RE.test(source)) return;
        if (MASTER_DETAIL_ALLOWED_FILES.has(file)) return;
        context.report({ node, messageId: "secondPane" });
      },
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (PANE_ELEMENTS.has(name)) {
          if (locate(node).overlay) {
            context.report({
              node,
              messageId: "paneInOverlay",
              data: { element: name },
            });
          }
          return;
        }
        if (!DETAIL_VIEW_ELEMENTS.has(name)) return;
        if (formContext.delete(`${name}@${node.loc.start.line}`)) return;
        const shell = enclosingElement(node, SLIDE_SHELLS);
        if (shell) {
          context.report({
            node,
            messageId: "detailInSlideOver",
            data: { element: name, shell },
          });
        }
      },
    };
  },
};

// --- bauart/no-edit-overlay --------------------------------------------------

// Overlays that carry their own title: `title` prop on the modals, the
// `<SlideOverTitle>` / `<DrawerTitle>` element in the slide-over family.
const EDIT_OVERLAY_TITLE_PROP_ELEMENTS = new Set([
  "Modal",
  "FormModal",
  "ChoiceModal",
]);
const EDIT_OVERLAY_TITLE_CHILD_ELEMENTS = new Set([
  "SlideOverTitle",
  "DrawerTitle",
]);
// `DatabaseFormModal mode="edit"` titles itself „… bearbeiten“ inside the kit.
const DATABASE_FORM_MODAL = "DatabaseFormModal";
// Word-bounded on unicode letters (see DELETE_LABEL_RE): „Personal
// bearbeiten“, „Rolle verwalten: Mila“ match, „Bearbeitung“ does not.
const EDIT_TITLE_RE = /(?:^|\P{L})(?:bearbeiten|verwalten)(?:\P{L}|$)/iu;

// Shrink-only per-match tolerance, keyed like ROW_ACTION_BASELINE: the static
// title text at its exact line, so a moved or reworded overlay cannot inherit
// the tolerance. Keys are repo-relative posix paths under src/. Line 0
// tolerates the title anywhere in its file — for titles assembled from
// several branches, whose line shifts with every edit above them.
const EDIT_OVERLAY_BASELINE = new Map(
  Object.entries({
    // Feldgruppe oder ganzes Objekt an einer Objektansicht, noch umzuziehen
    // (#3116, Folge-PRs über #3119): der Reiter selbst muss in den
    // Bearbeiten-Zustand wechseln.
    "src/app/[tenant]/(protected)/parent-announcements/page.tsx": [
      "Umfrage bearbeiten Neue Umfrage Elternbrief bearbeiten Neuer Elternbrief Elternmitteilung bearbeiten Neue Elternmitteilung@1098",
    ],
    "src/components/enrollment/admin-enrollment-detail.tsx": [
      "Betreuungsangebote bearbeiten@1562",
    ],
    "src/components/staff/abwesenheiten-tab.tsx": [
      "Urlaubsanspruch bearbeiten@1315",
    ],
    "src/components/staff/arbeitszeitmodell-tab.tsx": [
      "Arbeitszeitmodell bearbeiten@626",
    ],
    "src/components/students/care-weekly-plan-modal.tsx": [
      "Wochenplan bearbeiten@225",
    ],
    // Einträge ohne eigene Objektansicht (Termin, Schließtag,
    // Kalenderzeitraum, Jahrgangswechsel, Klassenlisteneintrag,
    // Anmeldephase, Betreuungsangebot, Ordner, Tagesinformation): die Spec
    // kennt für sie noch keine Bauart. Bis sie eine bekommen, bleibt der
    // Bestand stehen und wächst nicht.
    "src/app/[tenant]/(protected)/calendar/page.tsx": [
      "Termin bearbeiten Termin erstellen@918",
    ],
    "src/app/[tenant]/(protected)/database/students/class-list/page.tsx": [
      "edit Eintrag bearbeiten Klassenlisteneintrag anlegen@529",
    ],
    "src/components/database/grade-transitions/transition-editor.tsx": [
      "Jahrgangswechsel bearbeiten Neuer Jahrgangswechsel@243",
    ],
    "src/components/enrollment/care-offerings-editor.tsx": [
      "Betreuungsangebot duplizieren new Neues Betreuungsangebot Betreuungsangebot bearbeiten@936",
    ],
    "src/components/enrollment/phases-editor.tsx": [
      "Anmeldephase übertragen new Neue Anmeldephase Anmeldephase bearbeiten@853",
    ],
    "src/components/files/folder-modal.tsx": [
      "Ordner bearbeiten Neuer Ordner@131",
    ],
    "src/components/planning/closing-day-modal.tsx": [
      "Schließtag bearbeiten Schließtag anlegen@89",
    ],
    "src/components/staff-notices/staff-notice-modal.tsx": [
      "Tagesinformation bearbeiten Neue Tagesinformation@182",
    ],
    "src/components/timetable/calendar-period-modal.tsx": [
      "Kalenderzeitraum bearbeiten Kalenderzeitraum anlegen@380",
    ],
    // Kontoaktionen mit eigenem Ablauf (Begründung und Rückfrage bei der
    // Zwei-Faktor-Authentifizierung, Auflösen offener Zuordnungen bei der
    // Betreuung): keine Feldgruppe des Datensatzes, aus dem Kebab der
    // Personalakte geöffnet (#3116).
    "src/components/auth/mfa-admin-override-modal.tsx": [
      "Zwei-Faktor-Authentifizierung verwalten@314",
    ],
    "src/components/teachers/caregiver-capability-modal.tsx": [
      "resolve Zuordnungen auflösen: Betreuung verwalten:@268",
    ],
  }),
);

function normalizeTitle(chunks) {
  return chunks.join(" ").replaceAll(/\s+/g, " ").trim();
}

/** Static text of an attribute, or null when the element has none. */
function staticAttributeText(openingElement, name) {
  const attribute = jsxAttribute(openingElement, name);
  if (!attribute?.value) return null;
  const chunks = [];
  collectStaticStrings(attribute.value, chunks);
  return normalizeTitle(chunks);
}

const noEditOverlay = {
  meta: {
    type: "problem",
    docs: {
      description:
        "An object is edited in place: a tab switches into its edit state and back. No modal or slide-over titled „… bearbeiten“ / „… verwalten“ next to the object view.",
    },
    messages: {
      editOverlay:
        "Overlay „{{title}}“ ist ein Modal je Feldgruppe: Bearbeitet wird am Objekt, ein Reiter wechselt in den Bearbeiten-Zustand mit `EditActions` unten (BAUARTEN-SPEC Bauart 2 Regeln 3 und 4, #3116). Kontoaktionen mit eigenem Ablauf stehen im Kebab des Kopfes. Die Baseline in scripts/oxlint-plugin-bauart.mjs ist shrink-only.",
      editFormModal:
        "`DatabaseFormModal mode=\"edit\"` ist ein Modal für das ganze Objekt: Bearbeitet wird am Objekt, im Bearbeiten-Zustand des Reiters (BAUARTEN-SPEC Bauart 2 Regel 3, #3116). Die Baseline in scripts/oxlint-plugin-bauart.mjs ist shrink-only.",
    },
    schema: [],
  },
  create(context) {
    if (isExempt(context)) return {};
    const key = fileKey(context);
    if (OTHER_PORTAL_RE.test(key)) return {};
    const tolerated = new Set(EDIT_OVERLAY_BASELINE.get(key) ?? []);

    function report(node, title, messageId) {
      if (tolerated.delete(`${title}@${node.loc.start.line}`)) return;
      context.report({ node, messageId, data: { title } });
    }

    return {
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name === DATABASE_FORM_MODAL) {
          const mode = jsxAttribute(node, "mode");
          const chunks = [];
          if (mode?.value) collectStaticStrings(mode.value, chunks);
          if (chunks.includes("edit")) {
            report(node, "mode=edit", "editFormModal");
          }
          return;
        }
        if (EDIT_OVERLAY_TITLE_PROP_ELEMENTS.has(name)) {
          const title = staticAttributeText(node, "title");
          if (title && EDIT_TITLE_RE.test(title)) {
            report(node, title, "editOverlay");
          }
          return;
        }
        if (EDIT_OVERLAY_TITLE_CHILD_ELEMENTS.has(name)) {
          const chunks = [];
          collectChildText(node.parent, chunks);
          const title = normalizeTitle(chunks);
          if (EDIT_TITLE_RE.test(title)) report(node, title, "editOverlay");
        }
      },
    };
  },
};

export default {
  meta: { name: "bauart" },
  rules: {
    "one-delete-confirm": oneDeleteConfirm,
    "no-row-action-buttons": noRowActionButtons,
    "no-unconfirmed-destructive-click": noUnconfirmedDestructiveClick,
    "no-autosave": noAutosave,
    "no-toast-form-error": noToastFormError,
    "no-manage-surface-in-overlay": noManageSurfaceInOverlay,
    "one-detail-per-type": oneDetailPerType,
    "no-edit-overlay": noEditOverlay,
  },
};
