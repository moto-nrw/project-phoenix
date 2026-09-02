// UI-kit drift ratchet (issue #1629).
//
// Five hard-zero rules that stop drift away from the shared UI kit. The
// hand-rolled surface rule started with a shrink-only baseline (#2934) that
// was brought to zero and deleted in #2933; the first production match of
// any rule now fails `pnpm run check`:
//
//   ui-kit/no-generic-brand-colors  — generic Tailwind hues (bg-green-500 …)
//                                     used for brand semantics; use the brand
//                                     hex from LOCATION_COLORS or a kit
//                                     component (.claude/rules/frontend-ui-kit.md)
//   ui-kit/no-hand-rolled-overlay   — `fixed inset-0` overlays outside
//                                     src/components/ui/; use Modal / Drawer /
//                                     OverflowMenu from the kit
//   ui-kit/no-hand-rolled-surface   — hand-built card surfaces
//                                     (`rounded-xl/2xl` + `border` + `bg-white`)
//                                     outside src/components/ui/; use
//                                     moto-content-surface (cards),
//                                     moto-popover-surface (floating menus),
//                                     ChoiceTile (selectable rows) or a kit
//                                     surface component (issue #2933)
//   ui-kit/no-rounded-3xl           — off-scale surface radius; cards are
//                                     rounded-2xl (moto-content-surface)
//   ui-kit/require-checkbox-label   — every shared Checkbox is wrapped by a
//                                     label so its visible box is clickable
//
// Test/stories files are exempt from the class-string rules. A deliberate
// non-card match stays behind `oxlint-disable-next-line` with a reason; there
// is no baseline to grow.

const BRAND_COLOR_RE =
  /\b(?:text|bg|border(?:-[trblxyse])?|ring|outline|fill|stroke|from|via|to|divide|accent|caret|decoration|shadow)-(?:red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d+(?:\/(?:\d+|\[[^\]\s]+\]))?(?![\w-])/g;
const FIXED_RE = /\bfixed\b/;
const INSET_0_RE = /\binset-0\b/;
// A surface is three UNPREFIXED tokens: `print:border`, `sm:rounded-2xl` and
// `focus:bg-white` belong to another state, `border-0` / `border-none` /
// `border-b` draw no frame, and `bg-white/80` is not the solid card fill.
// `\b` accepted all of those because `-` and `:` are word boundaries.
const SURFACE_ROUNDED_RE = /(?:^|\s)rounded-(?:xl|2xl)(?=\s|$)/;
const SURFACE_BORDER_RE = /(?:^|\s)border(?:-[1-9]\d*)?(?=\s|$)/;
const SURFACE_BG_RE = /(?:^|\s)bg-white(?=\s|$)/;
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
        "Hand-rolled full-screen overlay ('{{match}}'). Use the kit Modal / Drawer / OverflowMenu instead of a bespoke fixed inset-0 layer.",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (key.startsWith(UI_KIT_DIR)) return {};

    const check = (node, text) => {
      if (!FIXED_RE.test(text) || !INSET_0_RE.test(text)) return;
      context.report({
        node,
        messageId: "handRolledOverlay",
        data: { match: "fixed … inset-0" },
      });
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

const isSurfaceCombo = (text) =>
  SURFACE_ROUNDED_RE.test(text) &&
  SURFACE_BORDER_RE.test(text) &&
  SURFACE_BG_RE.test(text);

const noHandRolledSurface = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-built card surfaces (`rounded-xl/2xl` + `border` + `bg-white`) outside the UI kit; use moto-content-surface, moto-popover-surface, ChoiceTile or a kit surface component (InfoCard, SectionCard).",
    },
    messages: {
      handRolledSurface:
        "Hand-rolled card surface ('{{match}}'). Use moto-content-surface (cards), moto-popover-surface (floating menus), ChoiceTile (selectable rows and tiles) or a kit surface component (InfoCard, SectionCard) instead. A deliberate non-card match may stay behind `// oxlint-disable-next-line ui-kit/no-hand-rolled-surface` with the reason recorded in the PR description (issue #2933).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (key.startsWith(UI_KIT_DIR)) return {};

    const check = (node, text) => {
      if (!isSurfaceCombo(text)) return;
      context.report({
        node,
        messageId: "handRolledSurface",
        data: { match: "rounded-xl/2xl … border … bg-white" },
      });
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
        if (chunks.some((chunk) => isSurfaceCombo(chunk))) {
          return; // already reported by the Literal/TemplateLiteral visitors
        }
        const combos = classCombinations(node.value) ?? [chunks.join(" ")];
        const rendered = combos.find((combo) => isSurfaceCombo(combo));
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

/**
 * `<ChoiceTile>` renders a native `<label>` unless its `as` prop says
 * otherwise, so a Checkbox inside the default tile already owns the whole
 * hit area. `as="button"` / `as="div"` tiles are not labels.
 */
function isLabelChoiceTile(openingElement) {
  if (jsxName(openingElement.name) !== "ChoiceTile") return false;
  const asAttribute = openingElement.attributes.find(
    (attribute) =>
      attribute.type === "JSXAttribute" &&
      attribute.name.type === "JSXIdentifier" &&
      attribute.name.name === "as",
  );
  if (!asAttribute) return true;
  return (
    asAttribute.value?.type === "Literal" && asAttribute.value.value === "label"
  );
}

function hasLabelAncestor(node) {
  for (let ancestor = node.parent; ancestor; ancestor = ancestor.parent) {
    if (ancestor.type !== "JSXElement") continue;
    if (
      jsxName(ancestor.openingElement.name) === "label" ||
      isLabelChoiceTile(ancestor.openingElement)
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
    "no-hand-rolled-surface": noHandRolledSurface,
    "no-rounded-3xl": noRounded3xl,
    "require-checkbox-label": requireCheckboxLabel,
  },
};
