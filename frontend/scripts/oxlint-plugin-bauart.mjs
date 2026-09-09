// Bauarten ratchet (BAUARTEN-SPEC.md, Abschnitt „Ratschen“).
//
// One hard-zero rule so far; the first production match fails
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
//                               is not a deletion and passes.
//
// Files under src/components/ui/ are exempt (ConfirmDeleteModal is itself
// built on Modal), as are tests and stories.

const UI_KIT_DIR_RE = /(?:^|\/)src\/components\/ui\//;
const EXEMPT_FILE_RE = /(?:\.(?:test|stories)\.)|(?:\.d\.ts$)/;

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

export default {
  meta: { name: "bauart" },
  rules: {
    "one-delete-confirm": oneDeleteConfirm,
  },
};
