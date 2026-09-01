// UI-kit drift ratchet (issue #1629).
//
// Four rules that stop drift away from the shared UI kit. Hand-rolled overlays
// tolerate existing stock via a shrink-only baseline; generic brand colors,
// rounded-3xl and unlabeled checkboxes are hard-zero:
//
//   ui-kit/no-generic-brand-colors  — generic Tailwind hues (bg-green-500 …)
//                                     used for brand semantics; use the brand
//                                     hex from LOCATION_COLORS or a kit
//                                     component (.claude/rules/frontend-ui-kit.md)
//   ui-kit/no-hand-rolled-overlay   — `fixed inset-0` overlays outside
//                                     src/components/ui/; use Modal / Drawer /
//                                     OverflowMenu from the kit
//   ui-kit/no-rounded-3xl           — off-scale surface radius; cards are
//                                     rounded-2xl (moto-content-surface)
//   ui-kit/require-checkbox-label   — every shared Checkbox is wrapped by a
//                                     label so its visible box is clickable
//
// The overlay baseline below is SHRINK-ONLY: matches may be removed when a file
// is migrated, but never added. Test/stories files are exempt from the
// class-string rules.

const OVERLAY_BASELINE_FILES = new Set([
  "src/app/operator/devices/page.tsx",
  "src/app/operator/persons/page.tsx",
  "src/app/operator/provisioning/soft-delete-shared.tsx",
  "src/app/operator/unregistered-tags/page.tsx",
  "src/components/auth/mfa-admin-override-modal.tsx",
  "src/components/background-wrapper.tsx",
  "src/components/dashboard/header/profile-dropdown.tsx",
  "src/components/enrollment/admin-enrollment-detail.tsx",
  "src/contexts/ToastContext.tsx",
]);

const OVERLAY_BASELINE = new Map([
  ["src/app/operator/devices/page.tsx", 1],
  ["src/app/operator/persons/page.tsx", 1],
  ["src/app/operator/provisioning/soft-delete-shared.tsx", 1],
  ["src/app/operator/unregistered-tags/page.tsx", 1],
  ["src/components/auth/mfa-admin-override-modal.tsx", 1],
  ["src/components/background-wrapper.tsx", 1],
  ["src/components/dashboard/header/profile-dropdown.tsx", 1],
  ["src/components/enrollment/admin-enrollment-detail.tsx", 1],
  ["src/contexts/ToastContext.tsx", 2],
]);

const BRAND_COLOR_RE =
  /\b(?:text|bg|border(?:-[trblxyse])?|ring|outline|fill|stroke|from|via|to|divide|accent|caret|decoration|shadow)-(?:red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d+(?:\/(?:\d+|\[[^\]\s]+\]))?(?![\w-])/g;
const FIXED_RE = /\bfixed\b/;
const INSET_0_RE = /\binset-0\b/;
const ROUNDED_3XL_RE = /\brounded-3xl\b/g;

const EXEMPT_FILE_RE = /(?:\.(?:test|stories)\.)|(?:\.d\.ts$)/;
const UI_KIT_DIR = "src/components/ui/";

/** Repo-relative posix path ("src/…"), or "" when unavailable. */
function fileKey(context) {
  const raw = String(context.physicalFilename ?? context.filename ?? "");
  const posix = raw.replaceAll("\\", "/");
  const idx = posix.lastIndexOf("/src/");
  return idx === -1 ? posix : posix.slice(idx + 1);
}

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

const MAX_CLASS_COMBINATIONS = 64;

function crossJoin(left, right) {
  const out = [];
  for (const a of left) {
    for (const b of right) {
      out.push(`${a} ${b}`);
      if (out.length > MAX_CLASS_COMBINATIONS) return null;
    }
  }
  return out;
}

/**
 * Possible rendered class strings of a className expression. Conditional and
 * logical branches contribute ALTERNATIVES instead of being joined, so
 * mutually exclusive literals (`cond ? "fixed …" : "… inset-0"`) cannot
 * combine into a phantom overlay, and strings inside a branch test
 * (`pos === "fixed" ? … : …`) never count as rendered classes. Returns null
 * when the combination count overflows; callers fall back to the joined
 * over-approximation.
 */
function classCombinations(node, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return [""];
  seen.add(node);

  if (node.type === "Literal") {
    return [typeof node.value === "string" ? node.value : ""];
  }
  if (node.type === "TemplateElement") return [node.value.raw];
  if (node.type === "ConditionalExpression") {
    const consequent = classCombinations(node.consequent, seen);
    const alternate = classCombinations(node.alternate, seen);
    if (!consequent || !alternate) return null;
    const merged = [...consequent, ...alternate];
    return merged.length > MAX_CLASS_COMBINATIONS ? null : merged;
  }
  if (node.type === "LogicalExpression") {
    const right = classCombinations(node.right, seen);
    if (!right) return null;
    if (node.operator === "&&") return ["", ...right];
    const left = classCombinations(node.left, seen);
    if (!left) return null;
    const merged = [...left, ...right];
    return merged.length > MAX_CLASS_COMBINATIONS ? null : merged;
  }

  let combos = [""];
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    const children = Array.isArray(value) ? value : [value];
    for (const child of children) {
      if (!child || typeof child !== "object") continue;
      const childCombos = classCombinations(child, seen);
      if (!childCombos) return null;
      combos = crossJoin(combos, childCombos);
      if (!combos) return null;
    }
  }
  return combos;
}

/**
 * Builds a hard-zero rule that reports string/template-literal chunks matching
 * `regex`.
 */
function makeClassStringRule({ regex, skipUiKit, docs, messageId, message }) {
  return {
    meta: {
      type: "problem",
      docs: { description: docs },
      messages: { [messageId]: message },
      schema: [],
    },
    create(context) {
      const key = fileKey(context);
      if (!key.startsWith("src/")) return {};
      if (EXEMPT_FILE_RE.test(key)) return {};
      if (skipUiKit && key.startsWith(UI_KIT_DIR)) return {};

      const check = (node, text) => {
        regex.lastIndex = 0;
        for (const match of text.matchAll(regex)) {
          context.report({ node, messageId, data: { match: match[0] } });
        }
      };

      return {
        Literal(node) {
          if (typeof node.value === "string") check(node, node.value);
        },
        TemplateLiteral(node) {
          for (const quasi of node.quasis) {
            check(node, quasi.value.raw);
          }
        },
      };
    },
  };
}

const noGenericBrandColors = makeClassStringRule({
  regex: BRAND_COLOR_RE,
  skipUiKit: false,
  docs: "Disallow generic Tailwind brand-color utilities; brand semantics use the LOCATION_COLORS hexes or a kit component.",
  messageId: "genericBrandColor",
  message:
    "Generic Tailwind hue '{{match}}…' is not a brand color. Use a moto-* semantic token, LOCATION_COLORS (~/lib/location-helper), or a kit component (see .claude/rules/frontend-ui-kit.md).",
});

const noHandRolledOverlay = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-rolled full-screen overlays (`fixed inset-0`) outside the UI kit; use Modal, Drawer, ConfirmationModal, or OverflowMenu.",
    },
    messages: {
      handRolledOverlay:
        "Hand-rolled full-screen overlay ('{{match}}'). Use the kit Modal / Drawer / OverflowMenu instead of a bespoke fixed inset-0 layer. The baseline in scripts/oxlint-plugin-ui-kit.mjs is shrink-only.",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (key.startsWith(UI_KIT_DIR)) return {};

    const baseline = OVERLAY_BASELINE_FILES.has(key)
      ? (OVERLAY_BASELINE.get(key) ?? 0)
      : 0;
    let seenMatches = 0;

    const check = (node, text) => {
      if (!FIXED_RE.test(text) || !INSET_0_RE.test(text)) return;

      seenMatches += 1;
      if (seenMatches > baseline) {
        context.report({
          node,
          messageId: "handRolledOverlay",
          data: { match: "fixed … inset-0" },
        });
      }
    };

    return {
      Literal(node) {
        if (typeof node.value === "string") check(node, node.value);
      },
      TemplateLiteral(node) {
        for (const quasi of node.quasis) {
          check(node, quasi.value.raw);
        }
      },
      JSXAttribute(node) {
        if (
          node.name.type !== "JSXIdentifier" ||
          node.name.name !== "className"
        )
          return;

        const chunks = [];
        collectStaticStrings(node.value, chunks);
        if (
          chunks.some((chunk) => FIXED_RE.test(chunk) && INSET_0_RE.test(chunk))
        ) {
          return; // already reported by the Literal/TemplateLiteral visitors
        }
        const combos = classCombinations(node.value) ?? [chunks.join(" ")];
        const rendered = combos.find(
          (combo) => FIXED_RE.test(combo) && INSET_0_RE.test(combo),
        );
        if (rendered !== undefined) check(node, rendered);
      },
    };
  },
};

const noRounded3xl = makeClassStringRule({
  regex: ROUNDED_3XL_RE,
  skipUiKit: false,
  docs: "Disallow rounded-3xl surfaces; the canonical card radius is rounded-2xl (moto-content-surface).",
  messageId: "rounded3xl",
  message:
    "rounded-3xl is off the brand radius scale. Cards/panels use rounded-2xl via moto-content-surface (see .claude/rules/frontend-ui-kit.md).",
});

function jsxName(node) {
  return node?.type === "JSXIdentifier" ? node.name : null;
}

function hasLabelAncestor(node) {
  for (let ancestor = node.parent; ancestor; ancestor = ancestor.parent) {
    if (
      ancestor.type === "JSXElement" &&
      jsxName(ancestor.openingElement.name) === "label"
    ) {
      return true;
    }
  }
  return false;
}

const requireCheckboxLabel = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Require the shared Checkbox and its visible text to share one native label hit area.",
    },
    messages: {
      missingLabel:
        "Wrap Checkbox and its visible text in one <label>. A sibling label names the hidden input but leaves the visible box unclickable.",
    },
    schema: [],
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (jsxName(node.name) !== "Checkbox" || hasLabelAncestor(node)) {
          return;
        }
        context.report({ node, messageId: "missingLabel" });
      },
    };
  },
};

export default {
  meta: { name: "ui-kit" },
  rules: {
    "no-generic-brand-colors": noGenericBrandColors,
    "no-hand-rolled-overlay": noHandRolledOverlay,
    "no-rounded-3xl": noRounded3xl,
    "require-checkbox-label": requireCheckboxLabel,
  },
};
