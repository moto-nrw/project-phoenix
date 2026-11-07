import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, normalize } from "node:path/posix";
import { fileURLToPath } from "node:url";

import {
  buttonRatchetAllowlist,
  buttonRatchetBaseline,
} from "./ui-kit-button-baseline.mjs";
import { importantOverrideBaseline } from "./ui-kit-important-override-baseline.mjs";

// UI-kit drift ratchet (issue #1629).
//
// Hard-zero rules that stop drift away from the shared UI kit (plus the
// shrink-only button ratchet at the end of this file, #5531). The
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
//                                     (`rounded-xl/2xl` + `border` + `bg-white`),
//                                     in the kit except SURFACE_KIT_CONTROLS; use
//                                     moto-content-surface (cards),
//                                     moto-popover-surface (floating menus),
//                                     ChoiceTile (selectable rows) or a kit
//                                     surface component (issue #2933)
//   ui-kit/no-rounded-3xl           — off-scale surface radius, also per
//                                     side/corner; cards are rounded-2xl
//                                     (moto-content-surface)
//   ui-kit/require-checkbox-label   — every shared Checkbox is wrapped by a
//                                     label so its visible box is clickable
//
// Four more rules belong to the tenant page scaffold (#2619):
//
//   ui-kit/no-tiny-text             — sub-12px text utilities; shrink-only
//                                     baseline
//   ui-kit/no-tenant-kicker         — no blue mini-heading in the tenant portal
//   ui-kit/no-hand-rolled-page-card — no `moto-content-surface … rounded-2xl`
//                                     spelled out in a tenant page/loading
//                                     file; use the kit cards (unlike
//                                     no-hand-rolled-surface, which bans the
//                                     plain-Tailwind card chain everywhere)
//   ui-kit/no-tabs-as-value-switcher — ui/Tabs is for content panels; values
//                                     go on SegmentedControl
//
// One more guards text on green fills (#5141):
//
//   ui-kit/green-fill-white-text    — opaque green surfaces carry white text,
//                                     only on the dark bg-moto-green-fill steps
//
// Two more keep dots where they belong (#5598, ADR 0149):
//
//   ui-kit/no-hand-rolled-dot       — dots and colour bars by hand outside
//                                     src/components/ui/; hard-zero
//   ui-kit/no-hand-rolled-portal-nav — sidebar and bottom bar of a portal only
//                                     from the kit; hard-zero (#5603)
//   ui-kit/no-status-dot-outside-plans — StatusDot only in legends and plans
//
// Three more catch classes that never render or miss contrast (#5577):
//
//   ui-kit/no-dead-surface-utility  — no colour/bg/shadow/ring/backdrop
//                                     utility beside the unlayered
//                                     moto-content-surface / -popover-surface
//   ui-kit/no-dynamic-color-class   — no `text-${…}` style colour classes;
//                                     Tailwind never generates them
//   ui-kit/no-white-on-warm-fill    — no white text on orange (SCHOOLYARD) or
//                                     amber (WARNING) fills
//
// Two more keep uploads and copyable values on the kit (#5599), hard-zero:
//
//   ui-kit/no-hand-rolled-dropzone  — no onDrop outside ui/dropzone.tsx; use
//                                     Dropzone
//   ui-kit/no-hand-rolled-copy-field — no framed <code> value and no
//                                     navigator.clipboard outside the kit and
//                                     useClipboardCopy; use CopyField
//
// One more keeps form fields on the kit (#5217):
//
//   ui-kit/no-kit-domain-import     — files in components/ui import no other
//                                     components and fetch no data (#5586)
//   ui-kit/no-raw-input             — no hand-built <input> or <textarea>
//                                     outside RAW_INPUT_KIT_PRIMITIVES; use Input,
//                                     Textarea, Checkbox, Radio, TimeField,
//                                     ConsentCheckbox or ColorSwatchInput.
//                                     Hidden and file inputs stay native.
//
// Two more keep the kit on one overlay system (#5579):
//
//   ui-kit/no-shadcn-tokens         — no shadcn theme tokens (bg-background,
//                                     text-muted-foreground …); bg-white/gray-*
//   ui-kit/no-raw-z-index           — no `z-[n]` inside src/components/ui/;
//                                     levels come from Z_LAYER
//
// One more keeps icon tiles on the kit (#5582):
//
//   ui-kit/no-hand-rolled-icon-tile — no hand-built icon tile (symbol on a
//                                     grey or tinted h-8…h-12 square/circle),
//                                     also inside src/components/ui/; use
//                                     IconTile or ConceptIconTile
//
// Three more keep one pill (#5584, hard-zero):
//
//   ui-kit/no-hand-rolled-pill      — tinted `span`/`div` pills outside StatusBadge
//   ui-kit/no-pill-dot              — a dot inside a pill (#2476)
//   ui-kit/no-hex-alpha-tint        — `${color}1A` tints in a pill's style
//
// One more keeps select fields on one look (#5580):
//
//   ui-kit/no-select-override       — no frame, fill, height or radius
//                                     classes on CustomSelect, MultiSelect
//                                     or ListboxDropdown outside the kit;
//                                     pick controlSize, pass layout only
//
// One more keeps the control height in the kit (#5605):
//
//   ui-kit/no-control-height-override — no `h-8`, `h-9` or `!size-*` on Input,
//                                     selects, DateRangePicker or OverflowMenu;
//                                     hard-zero
//
// Two more keep SectionCard the only card head (#5581):
//
//   ui-kit/no-important-override   — no `!` utility on a kit component; the
//                                     missing prop goes into the kit
//                                     (shrink-only baseline per file)
//   ui-kit/no-section-card-bleed   — no negative margin in a SectionCard
//                                     bodyClassName; use flush or footer
//
// The tiny-text baseline below is SHRINK-ONLY: matches may be removed when
// a file is migrated, but never added.

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
// Also the per-side and per-corner forms (`rounded-t-3xl`, `rounded-br-3xl`,
// logical `rounded-s-3xl` / `rounded-ss-3xl` …): the Drawer used them (#5579).
const ROUNDED_3XL_RE =
  /\brounded-(?:(?:t|b|l|r|tl|tr|bl|br|s|e|ss|se|es|ee)-)?3xl(?![\w-])/g;
// shadcn semantic colour tokens (`bg-background`, `text-muted-foreground`,
// `ring-ring` …). They resolve to slate tints from the old shadcn theme, not
// to the kit's white and gray scale; `bg-background` is not white (#5579).
const SHADCN_TOKEN_RE =
  /(?<![\w-])(?:bg|text|border(?:-[trblxyse])?|ring|ring-offset|outline|fill|stroke|divide|placeholder|from|via|to|caret|accent|decoration|shadow)-(?:background|foreground|muted(?:-foreground)?|accent(?:-foreground)?|popover(?:-foreground)?|card(?:-foreground)?|primary(?:-foreground)?|secondary(?:-foreground)?|destructive(?:-foreground)?|input|ring|border)(?![\w-])/g;
// Arbitrary stacking numbers (`z-[9999]`) inside the kit; the levels live in
// `Z_LAYER` (overlay-styles.ts), the one file this rule skips (#5579).
const RAW_Z_INDEX_RE = /(?<![\w-])z-\[[^\]\s]+\](?![\w-])/g;
const Z_LAYER_FILE = "src/components/ui/overlay-styles.ts";

const EXEMPT_FILE_RE = /(?:\.(?:test|stories)\.)|(?:\.d\.ts$)/;
const UI_KIT_DIR = "src/components/ui/";

/** Repo-relative posix path ("src/…"), or "" when unavailable. */
function fileKey(context) {
  const raw = String(context.physicalFilename ?? context.filename ?? "");
  const posix = raw.replaceAll("\\", "/");
  const idx = posix.lastIndexOf("/src/");
  return idx === -1 ? posix : posix.slice(idx + 1);
}

/** Parse `token@line` rows into immutable per-file occurrence snapshots. */
function parseLocationBaseline(source) {
  return new Map(
    source
      .trim()
      .split("\n")
      .map((row) => {
        const [file, encoded] = row.split("|");
        const occurrences = new Map();
        for (const location of encoded.split(" ")) {
          occurrences.set(location, (occurrences.get(location) ?? 0) + 1);
        }
        return [file, occurrences];
      }),
  );
}

function isBaselineMatch(baseline, key, match, line, seenOccurrences) {
  const location = `${match}@${line}`;
  const seen = seenOccurrences.get(location) ?? 0;
  seenOccurrences.set(location, seen + 1);
  return seen < (baseline.get(key)?.get(location) ?? 0);
}

function restrictMatchBaseline(baselineFiles, baseline) {
  return new Map([...baseline].filter(([file]) => baselineFiles.has(file)));
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

function findVariable(sourceCode, identifier) {
  let scope = sourceCode.getScope(identifier);
  while (scope) {
    const variable = scope.set.get(identifier.name);
    if (variable) return variable;
    scope = scope.upper;
  }
  return undefined;
}

function collectClassTemplateLiterals(
  node,
  templates,
  sourceCode,
  seen = new WeakSet(),
  seenVariables = new Set(),
) {
  if (!node || typeof node !== "object" || seen.has(node)) return;
  seen.add(node);

  if (node.type === "TemplateLiteral") {
    templates.push(node);
    return;
  }

  if (
    node.type === "JSXExpressionContainer" ||
    node.type === "ChainExpression" ||
    node.type === "TSAsExpression" ||
    node.type === "TSTypeAssertion" ||
    node.type === "TypeCastExpression"
  ) {
    collectClassTemplateLiterals(
      node.expression,
      templates,
      sourceCode,
      seen,
      seenVariables,
    );
    return;
  }

  if (node.type === "Identifier") {
    const variable = findVariable(sourceCode, node);
    if (!variable || seenVariables.has(variable)) return;
    seenVariables.add(variable);
    for (const definition of variable.defs) {
      if (
        definition.type !== "Variable" ||
        definition.node.type !== "VariableDeclarator" ||
        !definition.node.init
      )
        continue;
      collectClassTemplateLiterals(
        definition.node.init,
        templates,
        sourceCode,
        seen,
        seenVariables,
      );
    }
    return;
  }

  if (node.type === "ConditionalExpression") {
    collectClassTemplateLiterals(
      node.consequent,
      templates,
      sourceCode,
      seen,
      seenVariables,
    );
    collectClassTemplateLiterals(
      node.alternate,
      templates,
      sourceCode,
      seen,
      seenVariables,
    );
    return;
  }

  if (node.type === "LogicalExpression") {
    if (node.operator !== "&&") {
      collectClassTemplateLiterals(
        node.left,
        templates,
        sourceCode,
        seen,
        seenVariables,
      );
    }
    collectClassTemplateLiterals(
      node.right,
      templates,
      sourceCode,
      seen,
      seenVariables,
    );
    return;
  }

  if (node.type === "CallExpression") {
    for (const argument of node.arguments) {
      collectClassTemplateLiterals(
        argument,
        templates,
        sourceCode,
        seen,
        seenVariables,
      );
    }
    return;
  }

  if (node.type === "ArrayExpression") {
    for (const element of node.elements) {
      collectClassTemplateLiterals(
        element,
        templates,
        sourceCode,
        seen,
        seenVariables,
      );
    }
    return;
  }

  if (node.type === "ObjectExpression") {
    for (const property of node.properties) {
      if (property.type === "Property" && property.computed) {
        collectClassTemplateLiterals(
          property.key,
          templates,
          sourceCode,
          seen,
          seenVariables,
        );
      }
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
function classCombinations(
  node,
  seen = new WeakSet(),
  sourceCode,
  seenVariables = new Set(),
) {
  if (!node || typeof node !== "object" || seen.has(node)) return [""];
  seen.add(node);

  if (node.type === "Literal") {
    return [typeof node.value === "string" ? node.value : ""];
  }
  if (node.type === "TemplateElement") return [node.value.raw];
  if (node.type === "Identifier" && sourceCode) {
    const variable = findVariable(sourceCode, node);
    if (!variable || seenVariables.has(variable)) return [""];
    seenVariables.add(variable);
    const combinations = [];
    for (const definition of variable.defs) {
      if (
        definition.type !== "Variable" ||
        definition.node.type !== "VariableDeclarator" ||
        definition.parent?.kind !== "const" ||
        !definition.node.init
      )
        continue;
      const resolved = classCombinations(
        definition.node.init,
        seen,
        sourceCode,
        seenVariables,
      );
      if (!resolved) return null;
      combinations.push(...resolved);
      if (combinations.length > MAX_CLASS_COMBINATIONS) return null;
    }
    return combinations.length > 0 ? combinations : [""];
  }
  if (node.type === "ConditionalExpression") {
    const consequent = classCombinations(
      node.consequent,
      seen,
      sourceCode,
      seenVariables,
    );
    const alternate = classCombinations(
      node.alternate,
      seen,
      sourceCode,
      seenVariables,
    );
    if (!consequent || !alternate) return null;
    const merged = [...consequent, ...alternate];
    return merged.length > MAX_CLASS_COMBINATIONS ? null : merged;
  }
  if (node.type === "LogicalExpression") {
    const right = classCombinations(
      node.right,
      seen,
      sourceCode,
      seenVariables,
    );
    if (!right) return null;
    if (node.operator === "&&") return ["", ...right];
    const left = classCombinations(node.left, seen, sourceCode, seenVariables);
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
      const childCombos = classCombinations(
        child,
        seen,
        sourceCode,
        seenVariables,
      );
      if (!childCombos) return null;
      combos = crossJoin(combos, childCombos);
      if (!combos) return null;
    }
  }
  return combos;
}

/**
 * Builds a rule that reports string/template-literal chunks matching `regex`.
 * Hard-zero unless a shrink-only per-file `baseline` is given; then the
 * individual legacy matches recorded there are tolerated.
 */
function makeClassStringRule({
  regex,
  baseline = new Map(),
  skipUiKit,
  onlyUiKit = false,
  skipFiles = [],
  docs,
  messageId,
  message,
}) {
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
      if (onlyUiKit && !key.startsWith(UI_KIT_DIR)) return {};
      if (skipFiles.includes(key)) return {};
      const seenBaselineOccurrences = new Map();
      const check = (node, text) => {
        regex.lastIndex = 0;
        for (const match of text.matchAll(regex)) {
          // Nur die Tiny-Text-Regel hat eine Standort-Baseline. Die übrigen
          // Hard-zero-Regeln brauchen keine Quellposition und bleiben damit
          // auch mit den absichtlich schlanken AST-Knoten ihrer Unit-Tests
          // prüfbar.
          const baselineMatches = baseline.get(key);
          const line =
            baselineMatches && node.loc
              ? node.loc.start.line +
                (text.slice(0, match.index).match(/\n/g)?.length ?? 0)
              : undefined;
          if (
            !isBaselineMatch(
              baseline,
              key,
              match[0],
              line,
              seenBaselineOccurrences,
            )
          ) {
            context.report({ node, messageId, data: { match: match[0] } });
          }
        }
      };

      return {
        Literal(node) {
          if (typeof node.value === "string") check(node, node.value);
        },
        TemplateLiteral(node) {
          for (const quasi of node.quasis) {
            check(quasi, quasi.value.raw);
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

// Kit controls whose border and white fill belong to the control, not to a
// card (#5586). Every other kit file uses the kit surfaces.
const SURFACE_KIT_CONTROLS = new Map([
  [
    "src/components/ui/page-header/FilterButton.tsx",
    "Filterknopf der Kopfzeile",
  ],
  [
    "src/components/ui/page-header/FilterPanel.tsx",
    "Filtertafel: eigene Schattenstufe shadow-xl",
  ],
  ["src/components/ui/page-header/SearchBar.tsx", "Suchfeld der Kopfzeile"],
  ["src/components/ui/checkbox-card.tsx", "Auswahlzeile mit Häkchen"],
]);

const noHandRolledSurface = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-built card surfaces (`rounded-xl/2xl` + `border` + `bg-white`), in the UI kit too (#5586); use moto-content-surface, moto-popover-surface, ChoiceTile or a kit surface component (SectionCard).",
    },
    messages: {
      handRolledSurface:
        "Hand-rolled card surface ('{{match}}'). Use moto-content-surface (cards), moto-popover-surface (floating menus), ChoiceTile (selectable rows and tiles) or a kit surface component (SectionCard) instead. A deliberate non-card match may stay behind `// oxlint-disable-next-line ui-kit/no-hand-rolled-surface` with the reason recorded in the PR description (issue #2933).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (SURFACE_KIT_CONTROLS.has(key)) return {};

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

const noShadcnTokens = makeClassStringRule({
  regex: SHADCN_TOKEN_RE,
  skipUiKit: false,
  docs: "Disallow shadcn semantic colour tokens; the kit uses bg-white and the gray scale.",
  messageId: "shadcnToken",
  message:
    "{{match}} is a shadcn theme token (slate, not the kit palette). Use bg-white / gray-* or a moto-* utility (see .claude/rules/frontend-ui-kit.md).",
});

const noRawZIndex = makeClassStringRule({
  regex: RAW_Z_INDEX_RE,
  skipUiKit: false,
  onlyUiKit: true,
  skipFiles: [Z_LAYER_FILE],
  docs: "Disallow arbitrary z-index numbers in the UI kit; overlays take their level from Z_LAYER.",
  messageId: "rawZIndex",
  message:
    "{{match}} is a stacking number of its own. Take the level from Z_LAYER / Z_INDEX in overlay-styles.ts, or add a named level there.",
});

// ui-kit/green-fill-white-text — auf einer deckend grünen Fläche steht weiße
// Schrift, und zwar nur auf den dunklen Füllstufen (#5141). Zwei Fehlerbilder:
// dunkle Schrift (gray-900/950, black) auf irgendeinem deckenden Grün, und
// Weiß auf den hellen Grundtönen (moto-green 1,96:1, -hover 2,43:1, -light
// 1,76:1, -active, -vivid 3,25:1), die WCAG AA verfehlen. Getönte Flächen
// (bg-moto-green/10, -soft, -muted) tragen weiter dunkle oder grüne Schrift
// und fallen nicht unter die Regel. Geprüft wird je Element: ein String und
// jede gerenderte Kombination eines className-Ausdrucks.
const CLASS_VARIANT = String.raw`(?:[^\s:]+:)*!?`;
const classToken = (body) =>
  new RegExp(String.raw`(?:^|\s)${CLASS_VARIANT}${body}!?(?=\s|$)`);
const GREEN_OPAQUE_FILL_RE = classToken(
  String.raw`bg-moto-green(?:-(?:hover|active|light|vivid|strong|fill|fill-hover|fill-active))?`,
);
const GREEN_LIGHT_FILL_RE = classToken(
  String.raw`bg-moto-green(?:-(?:hover|active|light|vivid))?`,
);
const DARK_TEXT_RE = classToken(String.raw`text-(?:gray-(?:900|950)|black)`);
const WHITE_TEXT_RE = classToken("text-white");

function greenFillViolation(text) {
  if (GREEN_OPAQUE_FILL_RE.test(text) && DARK_TEXT_RE.test(text)) {
    return "darkText";
  }
  if (GREEN_LIGHT_FILL_RE.test(text) && WHITE_TEXT_RE.test(text)) {
    return "whiteOnLightGreen";
  }
  return null;
}

const greenFillWhiteText = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Green fills carry white text on the dark fill steps (bg-moto-green-fill, -fill-hover, -fill-active); no dark text on opaque green, no white text on the light brand greens.",
    },
    messages: {
      darkText:
        'Dark text on an opaque green fill. Green surfaces carry white text: use bg-moto-green-fill text-white (hover:bg-moto-green-fill-hover, active:bg-moto-green-fill-active), or Button variant="success" (#5141).',
      whiteOnLightGreen:
        "White text on a light brand green misses WCAG AA (moto-green 1.96:1). Use bg-moto-green-fill (4.84:1) with hover:bg-moto-green-fill-hover and active:bg-moto-green-fill-active (#5141).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};

    const check = (node, text) => {
      const messageId = greenFillViolation(text);
      if (messageId) context.report({ node, messageId });
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
        if (chunks.some((chunk) => greenFillViolation(chunk))) {
          return; // already reported by the Literal/TemplateLiteral visitors
        }
        const combos = classCombinations(node.value) ?? [chunks.join(" ")];
        const rendered = combos.find((combo) => greenFillViolation(combo));
        if (rendered !== undefined) check(node, rendered);
      },
    };
  },
};

// ui-kit/no-dead-surface-utility — moto-content-surface und
// moto-popover-surface stehen unlayered in globals.css und setzen
// border-color, background und box-shadow (die Inhaltsfläche zusätzlich
// backdrop-filter). Unlayered schlägt jede Tailwind-Utility aus
// @layer utilities, egal mit welcher Spezifität oder Variante. Eine
// Farb-, Hintergrund-, Schatten-, Ring- oder (nur Inhaltsfläche)
// Backdrop-Utility am selben Element ist deshalb tot: Die Fehlerkarte „Link
// ungültig“ blieb weiß statt rot, die verlinkte StatCard hob sich beim
// Überfahren nicht (#5577). Eine bewusst stärkere Utility (`shadow-md!`)
// gewinnt als !important und bleibt erlaubt. Hard-zero.
const CONTENT_SURFACE_RE = /(?:^|\s)moto-content-surface(?=\s|$)/;
const POPOVER_SURFACE_RE = /(?:^|\s)moto-popover-surface(?=\s|$)/;
// Varianten vorne abtrennen; Klammergruppen (`has-[:focus-visible]:`) dürfen
// selbst Doppelpunkte enthalten.
const VARIANT_PREFIX_RE = /^(?:(?:\[[^\]]*\]|[^:[])+:)*/;
const SURFACE_PAINT_RE =
  /^(?:bg-.+|border(?:-[trblxyse])?-(?!(?:[trblxyse](?:-|$)|\d|\[\d|px$|solid$|dashed$|dotted$|double$|hidden$|none$|collapse$|separate$|spacing-)).+|(?:inset-)?shadow(?:-.+)?|(?:inset-)?ring(?:-.+)?)$/;
const SURFACE_BACKDROP_RE = /^backdrop-.+$/;

function deadSurfaceTokens(text) {
  const content = CONTENT_SURFACE_RE.test(text);
  if (!content && !POPOVER_SURFACE_RE.test(text)) return [];
  return text.split(/\s+/).filter((token) => {
    if (token === "" || token.endsWith("!")) return false;
    const body = token.replace(VARIANT_PREFIX_RE, "");
    if (body.startsWith("!")) return false;
    return (
      SURFACE_PAINT_RE.test(body) || (content && SURFACE_BACKDROP_RE.test(body))
    );
  });
}

const noDeadSurfaceUtility = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow colour, background, shadow, ring and backdrop utilities on the same element as moto-content-surface / moto-popover-surface; the unlayered surface class overrides them.",
    },
    messages: {
      deadSurfaceUtility:
        "'{{tokens}}' wirkt neben {{surface}} nicht: Die Fläche steht unlayered in globals.css und schlägt Rahmenfarbe, Hintergrund, Schatten, Ring und (Inhaltsfläche) Backdrop jeder Utility. Utility streichen, die Fläche weglassen, wenn das Element anders aussehen soll, oder bewusst mit `!` erzwingen (#5577).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    const check = (node, text) => {
      const tokens = deadSurfaceTokens(text);
      if (tokens.length === 0) return;
      context.report({
        node,
        messageId: "deadSurfaceUtility",
        data: {
          tokens: tokens.join(" "),
          surface: CONTENT_SURFACE_RE.test(text)
            ? "moto-content-surface"
            : "moto-popover-surface",
        },
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
        if (chunks.some((chunk) => deadSurfaceTokens(chunk).length > 0)) {
          return; // already reported by the Literal/TemplateLiteral visitors
        }
        const combos = classCombinations(
          node.value,
          new WeakSet(),
          sourceCode,
        ) ?? [chunks.join(" ")];
        const rendered = combos.find(
          (combo) => deadSurfaceTokens(combo).length > 0,
        );
        if (rendered !== undefined) check(node, rendered);
      },
    };
  },
};

// ui-kit/no-dynamic-color-class — Tailwind erzeugt nur Klassen, die als
// ganzes Wort im Quelltext stehen. `text-${iconColor}` mit "blue-600" ergab
// keine Klasse, das Symbol blieb in der Textfarbe (#5577). Farbklassen stehen
// vollständig im Quelltext; ein Prop trägt die ganze Klasse oder einen Ton,
// den eine statische Tabelle auf Klassen abbildet. Hard-zero.
const DYNAMIC_COLOR_PREFIX_RE =
  /(?:^|[\s"'])(?:(?:\[[^\]]*\]|[^\s:[])+:)*!?(?:text|bg|border(?:-[trblxyse])?|ring)-$/;

const noDynamicColorClass = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow colour utilities assembled from a template expression (`text-${color}`); Tailwind only generates class names that appear whole in the source.",
    },
    messages: {
      dynamicColorClass:
        "'{{prefix}}${…}' wird zur Laufzeit zusammengesetzt; Tailwind erzeugt diese Klasse nicht. Die ganze Klasse in den Quelltext schreiben, z. B. über eine statische Tabelle Ton → Klasse (#5577).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode();
    const checkedTemplates = new WeakSet();

    const check = (node) => {
      if (checkedTemplates.has(node)) return;
      checkedTemplates.add(node);
      // Die letzte Quasi folgt auf keinen Ausdruck mehr.
      for (const quasi of node.quasis.slice(0, -1)) {
        const match = DYNAMIC_COLOR_PREFIX_RE.exec(quasi.value.raw);
        if (!match) continue;
        context.report({
          node: quasi,
          messageId: "dynamicColorClass",
          data: { prefix: match[0].trim() },
        });
      }
    };

    return {
      JSXAttribute(node) {
        if (
          node.name.type !== "JSXIdentifier" ||
          node.name.name !== "className"
        )
          return;

        const templates = [];
        collectClassTemplateLiterals(node.value, templates, sourceCode);
        for (const template of templates) check(template);
      },
    };
  },
};

// ui-kit/no-white-on-warm-fill — Orange (SCHOOLYARD, #F78C10: 2,4:1) und
// Amber (WARNING, #EAB308: 1,9:1) tragen keine weiße Schrift; WCAG AA
// verlangt 4,5:1. Geprüft werden die Klassen bg-moto-orange(-hover) und
// bg-moto-amber(-light) neben text-white sowie ein Inline-Hintergrund aus
// LOCATION_COLORS.SCHOOLYARD / .WARNING oder MOTO_COLOR_PALETTE.orange /
// .amber (base, hover, light) an einem Element mit text-white.
// Weiße Schrift steht auf dem -strong-Ton (orange-strong 5,6:1), sonst
// dunkle Schrift auf -soft (#5577). Hard-zero.
const WARM_FILL_RE = classToken(
  String.raw`bg-moto-(?:orange(?:-hover)?|amber(?:-light)?)`,
);
const WARM_LOCATION_KEYS = new Set(["SCHOOLYARD", "WARNING"]);

const WARM_PALETTE_TONES = new Set(["orange", "amber"]);
const WARM_PALETTE_STEPS = new Set(["base", "hover", "light"]);

const memberName = (node) =>
  node?.type === "MemberExpression" &&
  !node.computed &&
  node.property?.type === "Identifier"
    ? node.property.name
    : null;

/** MOTO_COLOR_PALETTE.orange.base / .amber.light … */
function isWarmPaletteColor(node) {
  const tone = node?.object;
  return (
    WARM_PALETTE_STEPS.has(memberName(node)) &&
    WARM_PALETTE_TONES.has(memberName(tone)) &&
    tone.object?.type === "Identifier" &&
    tone.object.name === "MOTO_COLOR_PALETTE"
  );
}

function isWarmLocationColor(node) {
  if (isWarmPaletteColor(node)) return true;
  return (
    node?.type === "MemberExpression" &&
    !node.computed &&
    node.object?.type === "Identifier" &&
    node.object.name === "LOCATION_COLORS" &&
    node.property?.type === "Identifier" &&
    WARM_LOCATION_KEYS.has(node.property.name)
  );
}

function hasWarmInlineBackground(openingElement) {
  const style = jsxAttribute(openingElement, "style");
  const expression =
    style?.value?.type === "JSXExpressionContainer"
      ? style.value.expression
      : null;
  if (expression?.type !== "ObjectExpression") return false;
  return expression.properties.some((property) => {
    if (property.type !== "Property") return false;
    const name =
      property.key.type === "Identifier"
        ? property.key.name
        : property.key.value;
    return (
      (name === "backgroundColor" || name === "background") &&
      isWarmLocationColor(property.value)
    );
  });
}

const whiteOnWarmFill = (text) =>
  WARM_FILL_RE.test(text) && WHITE_TEXT_RE.test(text);

const noWhiteOnWarmFill = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow white text on the orange (SCHOOLYARD) and amber (WARNING) fills; both miss WCAG AA against white.",
    },
    messages: {
      whiteOnWarmFill:
        "Weiße Schrift auf Orange/Amber verfehlt WCAG AA (Orange 2,4:1, Amber 1,9:1). Weiß nur auf dem -strong-Ton (bg-moto-orange-strong 5,6:1), sonst dunkle -strong-Schrift auf -soft (#5577).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    const check = (node, text) => {
      if (whiteOnWarmFill(text)) {
        context.report({ node, messageId: "whiteOnWarmFill" });
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
        if (chunks.some((chunk) => whiteOnWarmFill(chunk))) return;
        const combos = classCombinations(
          node.value,
          new WeakSet(),
          sourceCode,
        ) ?? [chunks.join(" ")];
        const rendered = combos.find((combo) => whiteOnWarmFill(combo));
        if (rendered !== undefined) check(node, rendered);
      },
      JSXOpeningElement(node) {
        if (!hasWarmInlineBackground(node)) return;
        const className = jsxAttribute(node, "className");
        if (!className?.value) return;
        const chunks = [];
        collectStaticStrings(className.value, chunks);
        if (chunks.some((chunk) => WHITE_TEXT_RE.test(chunk))) {
          context.report({ node, messageId: "whiteOnWarmFill" });
        }
      },
    };
  },
};

// ui-kit/no-tiny-text — Schrift unter 12 px (text-[9px], text-[10px],
// text-[11px]) ist unter dem Typo-Boden des Portals: text-xs (12 px) ist die
// Untergrenze und selbst die ist Versalien-Labels vorbehalten, nie Werten,
// Saetzen oder Bedienelementen (TENANT-PAGE-SPEC, "Typo-Boden"). Der Bestand
// ist als Shrink-only-Baseline eingefroren; neue Winzschrift kommt nicht dazu.
const TINY_TEXT_RE = /\btext-\[(?:9|10|11)px\]/g;

const TINY_TEXT_BASELINE_FILES = new Set([
  "src/app/[tenant]/(protected)/calendar/page.tsx",
  "src/app/[tenant]/(protected)/meal-plan/page.tsx",
  "src/app/[tenant]/(protected)/time-tracking/week-chart.tsx",
  "src/app/help/nfc/erste-schritte/page.tsx",
  "src/app/operator/provisioning/soft-delete-shared.tsx",
  "src/components/auth/role-permission-management-modal.tsx",
  "src/components/dashboard/header/reminders-bell.tsx",
  "src/components/dashboard/sidebar.tsx",
  "src/components/enrollment/admin-enrollments-list.tsx",
  "src/components/enrollment/enrollment-form.tsx",
  "src/components/files/files-page.tsx",
  "src/components/guardians/guardian-contact-actions.tsx",
  "src/components/guardians/guardian-list.tsx",
  "src/components/parent/calendar/parent-calendar-page.tsx",
  "src/components/parent/parent-meal-plan-page.tsx",
  "src/components/staff/absence-request-row.tsx",
  "src/components/staff/dienstplan-resource-grid.tsx",
  "src/components/staff/staff-session-table.tsx",
  "src/components/students/care-schedule-manager.tsx",
  "src/components/students/planned-status-days-modal.tsx",
  "src/components/students/school-checkin-fab.tsx",
  "src/components/time-tracking/edit-history-accordion.tsx",
  "src/components/time-tracking/vacation-request-modal.tsx",
  "src/components/timetable/bulk-substitution-modal.tsx",
  "src/components/timetable/event-form/multi-select-field.tsx",
  "src/components/timetable/event-form/step-termin.tsx",
  "src/components/timetable/gap-jump-list.tsx",
  "src/components/timetable/instance-block.tsx",
  "src/components/timetable/instance-detail-modal.tsx",
  "src/components/timetable/month-planner-grid.tsx",
  "src/components/timetable/period-switcher-dropdown.tsx",
  "src/components/timetable/staff-pool-slide-over.tsx",
  "src/components/timetable/substitution-person-card.tsx",
  "src/components/timetable/substitution-slide-over.tsx",
  "src/components/timetable/template-card.tsx",
  "src/components/timetable/vertretung-day-list.tsx",
  "src/components/timetable/weekly-calendar-grid.tsx",
  "src/components/ui/avatar.tsx",
  "src/components/ui/planning/coverage-indicator.tsx",
  "src/components/ui/location-badge.tsx",
  "src/components/ui/notification-badge.tsx",
  "src/components/ui/page-header/FilterButton.tsx",
  "src/components/ui/page-header/OverflowMenu.tsx",
  "src/components/ui/planning/plan-block.tsx",
  "src/components/ui/planning/plan-legend.tsx",
  "src/components/ui/presence-badge.tsx",
]);

const TINY_TEXT_BASELINE = parseLocationBaseline(`
src/app/[tenant]/(protected)/time-tracking/week-chart.tsx|text-[10px]@198
src/app/help/nfc/erste-schritte/page.tsx|text-[11px]@219 text-[11px]@263
src/app/operator/provisioning/soft-delete-shared.tsx|text-[11px]@231
src/components/auth/role-permission-management-modal.tsx|text-[10px]@330 text-[11px]@375
src/components/dashboard/header/reminders-bell.tsx|text-[11px]@43 text-[10px]@99
src/components/enrollment/admin-enrollments-list.tsx|text-[11px]@978
src/components/enrollment/enrollment-form.tsx|text-[11px]@2189
src/components/files/files-page.tsx|text-[11px]@180 text-[11px]@292
src/components/guardians/guardian-contact-actions.tsx|text-[10px]@141
src/components/guardians/guardian-list.tsx|text-[10px]@462
src/components/parent/calendar/parent-calendar-page.tsx|text-[11px]@744 text-[11px]@809
src/components/parent/parent-meal-plan-page.tsx|text-[11px]@461 text-[11px]@546
src/components/staff/absence-request-row.tsx|text-[11px]@107
src/components/staff/dienstplan-resource-grid.tsx|text-[11px]@488 text-[11px]@584 text-[11px]@593 text-[11px]@600
src/components/staff/staff-session-table.tsx|text-[11px]@1194
src/components/students/care-schedule-manager.tsx|text-[10px]@1232 text-[11px]@1393 text-[11px]@1482 text-[11px]@1492 text-[11px]@1520
src/components/students/planned-status-days-modal.tsx|text-[10px]@936 text-[11px]@936
src/components/students/school-checkin-fab.tsx|text-[10px]@199
src/components/time-tracking/edit-history-accordion.tsx|text-[10px]@118 text-[10px]@139 text-[10px]@150
src/components/time-tracking/vacation-request-modal.tsx|text-[11px]@374
src/components/timetable/bulk-substitution-modal.tsx|text-[11px]@529
src/components/timetable/event-form/multi-select-field.tsx|text-[11px]@149 text-[11px]@310
src/components/timetable/event-form/step-termin.tsx|text-[10px]@161 text-[11px]@231 text-[11px]@264 text-[11px]@341 text-[11px]@347 text-[11px]@395
src/components/timetable/gap-jump-list.tsx|text-[10px]@108 text-[11px]@131 text-[11px]@146
src/components/timetable/instance-block.tsx|text-[10px]@298 text-[10px]@332 text-[10px]@339 text-[10px]@349
src/components/timetable/instance-detail-modal.tsx|text-[11px]@1359 text-[11px]@1385 text-[11px]@1561 text-[10px]@1670 text-[11px]@1693
src/components/timetable/month-planner-grid.tsx|text-[11px]@67 text-[11px]@143 text-[11px]@152 text-[11px]@173 text-[9px]@204 text-[10px]@238
src/components/timetable/period-switcher-dropdown.tsx|text-[11px]@199 text-[10px]@215 text-[11px]@224 text-[10px]@266 text-[10px]@290 text-[10px]@363 text-[10px]@377 text-[10px]@389 text-[11px]@397 text-[11px]@404
src/components/timetable/staff-pool-slide-over.tsx|text-[10px]@404 text-[11px]@471
src/components/timetable/substitution-person-card.tsx|text-[11px]@56 text-[11px]@334 text-[11px]@341 text-[11px]@348 text-[11px]@356 text-[11px]@368
src/components/timetable/substitution-slide-over.tsx|text-[11px]@748 text-[11px]@752
src/components/timetable/template-card.tsx|text-[11px]@131 text-[10px]@201
src/components/timetable/vertretung-day-list.tsx|text-[11px]@351 text-[11px]@391 text-[11px]@398 text-[11px]@419 text-[11px]@425
src/components/timetable/weekly-calendar-grid.tsx|text-[10px]@456 text-[11px]@456 text-[11px]@461 text-[10px]@484 text-[10px]@519 text-[11px]@519
src/components/ui/avatar.tsx|text-[10px]@32
src/components/ui/planning/coverage-indicator.tsx|text-[11px]@61 text-[11px]@163
src/components/ui/location-badge.tsx|text-[10px]@282
src/components/ui/notification-badge.tsx|text-[10px]@15
src/components/ui/page-header/FilterButton.tsx|text-[10px]@61
src/components/ui/page-header/OverflowMenu.tsx|text-[10px]@463 text-[11px]@535
src/components/ui/planning/plan-block.tsx|text-[11px]@154 text-[11px]@175
src/components/ui/planning/plan-legend.tsx|text-[11px]@106
src/components/ui/presence-badge.tsx|text-[10px]@200
`);

const noTinyText = makeClassStringRule({
  regex: TINY_TEXT_RE,
  baseline: restrictMatchBaseline(TINY_TEXT_BASELINE_FILES, TINY_TEXT_BASELINE),
  skipUiKit: false,
  docs: "Disallow sub-12px text utilities; the portal type floor is text-xs, and only for uppercase labels.",
  messageId: "tinyText",
  message:
    "'{{match}}' liegt unter dem Typo-Boden (12 px). Werte, Saetze und Bedienelemente sind text-sm oder groesser; text-xs nur fuer Versalien-Labels. Die Baseline in scripts/oxlint-plugin-ui-kit.mjs ist shrink-only.",
});
// ui-kit/no-tenant-kicker — die blaue Mini-Überschrift über einer Überschrift
// ist im Tenant-Portal abgeschafft (sie trug sechs verschiedene Bedeutungen im
// selben Slot). Kein Baseline-Eintrag: der Bestand ist in derselben PR auf
// null gebracht. Eltern-, Schul- und Operator-Portal sind nicht Teil dieser
// Umstellung und bekommen ihren eigenen Durchgang.
const OTHER_PORTALS_RE =
  /(^|\/)(components\/parent|app\/parents|components\/school|app\/school|components\/class-day|components\/operator|app\/operator)\//;

const noTenantKicker = {
  meta: {
    docs: {
      description:
        "Disallow the blue mini-heading (kicker) outside the parents portal.",
    },
    messages: {
      kicker:
        "Keine Mini-Überschrift über dem Titel. Wo man ist, sagen Brotkrumen und Seitenleiste (siehe .claude/rules/frontend-ui-kit.md, Abschnitt Page scaffolding).",
    },
  },
  create(context) {
    const filename = context.filename ?? context.getFilename?.() ?? "";
    if (OTHER_PORTALS_RE.test(filename.replace(/\\/g, "/"))) return {};
    if (/\.(test|stories)\.[jt]sx?$/.test(filename)) return {};

    return {
      JSXAttribute(node) {
        if (node.name?.name === "kicker") {
          context.report({ node, messageId: "kicker" });
        }
      },
    };
  },
};

// ui-kit/no-hand-rolled-page-card — eine Seite baut keine Kartenfläche mehr aus
// einer eigenen Klassenkette. Für jede Form gibt es ein Kit-Bauteil:
// SectionCard (Fläche mit oder ohne Kopf), TileCard (anklickbare Kachel),
// StatCard (Kennzahl), TenantPageHeaderSkeleton (Ladezustand). Die Regel
// greift nur in Seitendateien des Tenant- und des Träger-Portals (#5146) —
// Kit- und Komponentendateien dürfen die Fläche definieren, sie sind die
// Quelle.
const TENANT_PAGE_FILE_RE =
  /app\/(?:\[tenant\]\/\(protected\)|traeger)\/(?:.*\/)?(page|loading)\.tsx$/;
// Nur die KARTE ist gemeint: `moto-content-surface` zusammen mit dem
// Kartenradius. Dieselbe Klasse an einer Pille oder einem Auswahlfeld
// (rounded-full, h-9) ist eine Bedienfläche und bleibt erlaubt.
const HANDROLLED_SURFACE_RE =
  /moto-content-surface[^"'`]*rounded-2xl|rounded-2xl[^"'`]*moto-content-surface/;

const noHandRolledPageCard = {
  meta: {
    docs: {
      description:
        "Disallow hand-rolled card surfaces in tenant page files; use the kit cards.",
    },
    messages: {
      surface:
        "Keine eigene Kartenfläche in einer Seite. SectionCard (Fläche), TileCard (anklickbare Kachel), StatCard (Kennzahl) oder TenantPageHeaderSkeleton nehmen (siehe .claude/rules/frontend-ui-kit.md, Abschnitt Page scaffolding).",
    },
  },
  create(context) {
    const filename = (
      context.filename ??
      context.getFilename?.() ??
      ""
    ).replace(/\\/g, "/");
    if (!TENANT_PAGE_FILE_RE.test(filename)) return {};
    if (/\.(test|stories)\.[jt]sx?$/.test(filename)) return {};

    return {
      Literal(node) {
        if (
          typeof node.value === "string" &&
          HANDROLLED_SURFACE_RE.test(node.value)
        ) {
          context.report({ node, messageId: "surface" });
        }
      },
      TemplateElement(node) {
        const raw = node.value?.cooked ?? node.value?.raw ?? "";
        if (HANDROLLED_SURFACE_RE.test(raw)) {
          context.report({ node, messageId: "surface" });
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
        if (chunks.some((chunk) => HANDROLLED_SURFACE_RE.test(chunk))) {
          return; // already reported by the Literal/TemplateElement visitors
        }
        const combos = classCombinations(node.value) ?? [chunks.join(" ")];
        const rendered = combos.find((combo) =>
          HANDROLLED_SURFACE_RE.test(combo),
        );
        if (rendered !== undefined) {
          context.report({ node, messageId: "surface" });
        }
      },
    };
  },
};

// ui-kit/no-tabs-as-value-switcher — `ui/Tabs` ist Radix und schaltet
// INHALTE; eine Wertauswahl (Woche/Monat, A/B, Umfang) gehört auf
// SegmentedControl. Beides nebeneinander erzeugt zwei Bauarten für dieselbe
// Geste, und die Radix-Variante schaltet auf mousedown, was Klick-Tests still
// ins Leere laufen lässt.
const VALUE_SWITCHER_LABELS = new Set([
  "Woche",
  "Monat",
  "Tag",
  "Halbjahr",
  "Serien",
  "Woche A",
  "Woche B",
  "Ganzer Tag",
  "Dieser Block",
]);
const VALUE_SWITCHER_VALUES = new Set([
  "week",
  "month",
  "day",
  "half-year",
  "series",
  "week-a",
  "week-b",
  "whole-day",
  "this-block",
]);
const UI_KIT_TABS_MODULES = new Set([
  "~/components/ui/tabs",
  "@/components/ui/tabs",
]);

function isUiKitTabsModule(source, filename) {
  if (UI_KIT_TABS_MODULES.has(source)) return true;
  if (!source.startsWith(".")) return false;
  const posixFilename = filename.replaceAll("\\", "/");
  const sourceStart = posixFilename.lastIndexOf("/src/");
  const importer =
    sourceStart === -1 ? posixFilename : posixFilename.slice(sourceStart + 1);
  return (
    normalize(`${dirname(importer)}/${source}`) === "src/components/ui/tabs"
  );
}

function collectStaticTabText(children, texts) {
  for (const child of children) {
    if (child.type === "JSXText") {
      texts.push(child.value);
    } else if (child.type === "JSXElement" || child.type === "JSXFragment") {
      collectStaticTabText(child.children, texts);
    } else if (child.type === "JSXExpressionContainer") {
      collectStaticStrings(child.expression, texts);
    }
  }
}

function isTabsElement(node, tabsNames, tabsPrimitiveNamespaces) {
  const name = node.openingElement?.name;
  if (tabsNames.has(jsxName(name))) return true;
  return (
    name?.type === "JSXMemberExpression" &&
    tabsPrimitiveNamespaces.has(jsxName(name.object)) &&
    jsxName(name.property) === "Root"
  );
}

function isTabsTriggerElement(node, tabsTriggerNames, tabsPrimitiveNamespaces) {
  const name = node.openingElement?.name;
  if (tabsTriggerNames.has(jsxName(name))) return true;
  return (
    name?.type === "JSXMemberExpression" &&
    tabsPrimitiveNamespaces.has(jsxName(name.object)) &&
    jsxName(name.property) === "Trigger"
  );
}

function isTabsContentElement(node, tabsContentNames, tabsPrimitiveNamespaces) {
  const name = node.openingElement?.name;
  if (tabsContentNames.has(jsxName(name))) return true;
  return (
    name?.type === "JSXMemberExpression" &&
    tabsPrimitiveNamespaces.has(jsxName(name.object)) &&
    jsxName(name.property) === "Content"
  );
}

function hasTabsContent(
  node,
  tabsNames,
  tabsContentNames,
  tabsPrimitiveNamespaces,
  seen = new WeakSet(),
  isRoot = true,
) {
  if (!node || typeof node !== "object" || seen.has(node)) return false;
  if (node.type === "JSXElement") {
    if (isTabsContentElement(node, tabsContentNames, tabsPrimitiveNamespaces)) {
      return true;
    }
    if (!isRoot && isTabsElement(node, tabsNames, tabsPrimitiveNamespaces)) {
      return false;
    }
  }
  seen.add(node);

  return Object.entries(node).some(([key, value]) => {
    if (key === "parent") return false;
    if (Array.isArray(value)) {
      return value.some((child) =>
        hasTabsContent(
          child,
          tabsNames,
          tabsContentNames,
          tabsPrimitiveNamespaces,
          seen,
          false,
        ),
      );
    }
    return hasTabsContent(
      value,
      tabsNames,
      tabsContentNames,
      tabsPrimitiveNamespaces,
      seen,
      false,
    );
  });
}

function isTabsWithoutContent(
  node,
  tabsNames,
  tabsContentNames,
  tabsPrimitiveNamespaces,
) {
  for (let ancestor = node.parent; ancestor; ancestor = ancestor.parent) {
    if (
      ancestor.type === "JSXElement" &&
      isTabsElement(ancestor, tabsNames, tabsPrimitiveNamespaces)
    ) {
      return !hasTabsContent(
        ancestor,
        tabsNames,
        tabsContentNames,
        tabsPrimitiveNamespaces,
      );
    }
  }
  return false;
}

const noTabsAsValueSwitcher = {
  meta: {
    docs: {
      description:
        "Disallow ui/Tabs for value choices; those belong on SegmentedControl.",
    },
    messages: {
      valueSwitcher:
        "Wertauswahl gehört auf SegmentedControl, nicht auf ui/Tabs (siehe .claude/rules/frontend-ui-kit.md, Abschnitt Page scaffolding).",
    },
  },
  create(context) {
    const filename = (
      context.filename ??
      context.getFilename?.() ??
      ""
    ).replace(/\\/g, "/");
    if (OTHER_PORTALS_RE.test(filename)) return {};
    if (/\.(test|stories)\.[jt]sx?$/.test(filename)) return {};
    const tabsNames = new Set();
    const tabsTriggerNames = new Set();
    const tabsContentNames = new Set();
    const tabsPrimitiveNamespaces = new Set();

    return {
      ImportDeclaration(node) {
        const source = node.source.value;
        if (typeof source !== "string") return;

        for (const specifier of node.specifiers) {
          if (isUiKitTabsModule(source, filename)) {
            if (specifier.type !== "ImportSpecifier") continue;
            if (specifier.imported.name === "Tabs") {
              tabsNames.add(specifier.local.name);
            } else if (specifier.imported.name === "TabsTrigger") {
              tabsTriggerNames.add(specifier.local.name);
            } else if (specifier.imported.name === "TabsContent") {
              tabsContentNames.add(specifier.local.name);
            }
            continue;
          }
          if (source !== "@radix-ui/react-tabs") continue;
          if (specifier.type === "ImportSpecifier") {
            if (specifier.imported.name === "Root") {
              tabsNames.add(specifier.local.name);
            } else if (specifier.imported.name === "Trigger") {
              tabsTriggerNames.add(specifier.local.name);
            } else if (specifier.imported.name === "Content") {
              tabsContentNames.add(specifier.local.name);
            }
          }
          if (specifier.type === "ImportNamespaceSpecifier") {
            tabsPrimitiveNamespaces.add(specifier.local.name);
          }
        }
      },
      JSXElement(node) {
        if (
          !isTabsTriggerElement(node, tabsTriggerNames, tabsPrimitiveNamespaces)
        )
          return;
        const texts = [];
        collectStaticTabText(node.children, texts);
        if (
          texts.some((text) => {
            const token = text.trim();
            return (
              VALUE_SWITCHER_LABELS.has(token) ||
              VALUE_SWITCHER_VALUES.has(token)
            );
          }) ||
          isTabsWithoutContent(
            node,
            tabsNames,
            tabsContentNames,
            tabsPrimitiveNamespaces,
          )
        ) {
          context.report({ node, messageId: "valueSwitcher" });
        }
      },
    };
  },
};

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

// --- ui-kit/no-kit-domain-import ---------------------------------------------
//
// The kit works in every portal, so it knows no domain: a file under
// components/ui/ imports nothing from other component folders and loads no
// data. Context arrives through props and slots (#5586).

const KIT_DATA_HOOK_RE = /^useSWR/;

function kitImportLeavesKit(source, key) {
  if (source === "swr" || source.startsWith("swr/")) return true;
  const alias = /^[~@]\/(.*)$/.exec(source);
  let target;
  if (alias) {
    target = `src/${alias[1]}`;
  } else if (source.startsWith(".")) {
    const parts = key.split("/").slice(0, -1);
    for (const part of source.split("/")) {
      if (part === "..") parts.pop();
      else if (part !== ".") parts.push(part);
    }
    target = parts.join("/");
  } else {
    return false;
  }
  if (target.startsWith(UI_KIT_DIR)) return false;
  return target.startsWith("src/components/") || target.startsWith("src/app/");
}

const noKitDomainImport = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Files under components/ui import no domain components and load no data.",
    },
    messages: {
      domainImport:
        "Das Kit importiert keine Fachlogik ('{{source}}'). Baustein nach components/ui ziehen oder Kontext über Props oder Slots hereinreichen (#5586, .claude/rules/frontend-ui-kit.md).",
      dataAccess:
        "Das Kit lädt keine Daten ('{{name}}'). Die Daten kommen über Props vom fachlichen Aufrufer (#5586, .claude/rules/frontend-ui-kit.md).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith(UI_KIT_DIR) || EXEMPT_FILE_RE.test(key)) return {};

    const checkSource = (node, source) => {
      if (typeof source !== "string" || !kitImportLeavesKit(source, key))
        return;
      context.report({ node, messageId: "domainImport", data: { source } });
    };
    const checkDeclaration = (node) => checkSource(node, node.source?.value);

    return {
      ImportDeclaration: checkDeclaration,
      ExportNamedDeclaration: checkDeclaration,
      ExportAllDeclaration: checkDeclaration,
      ImportExpression(node) {
        checkSource(node, node.source?.value);
      },
      CallExpression(node) {
        const { callee } = node;
        const name =
          callee.type === "Identifier"
            ? callee.name
            : callee.type === "MemberExpression" &&
                !callee.computed &&
                callee.object.type === "Identifier" &&
                (callee.object.name === "window" ||
                  callee.object.name === "globalThis")
              ? callee.property.name
              : null;
        if (name === "fetch" || (name && KIT_DATA_HOOK_RE.test(name))) {
          context.report({ node, messageId: "dataAccess", data: { name } });
        }
      },
    };
  },
};

// --- ui-kit/no-raw-input ----------------------------------------------------

// A native input that has no kit counterpart: a value carried in the form
// (`hidden`) and the file dialog a kit button opens (`file`).
const NATIVE_INPUT_TYPES = new Set(["hidden", "file"]);

function staticInputType(openingElement) {
  const attribute = openingElement.attributes.find(
    (candidate) =>
      candidate.type === "JSXAttribute" &&
      candidate.name.type === "JSXIdentifier" &&
      candidate.name.name === "type",
  );
  const value = attribute?.value;
  if (value?.type === "Literal") return String(value.value);
  if (
    value?.type === "JSXExpressionContainer" &&
    value.expression.type === "Literal"
  ) {
    return String(value.expression.value);
  }
  return null;
}

// Kit files that are the primitive themselves: every other kit file composes
// them (#5586). Each entry names what the native element is for.
const RAW_INPUT_KIT_PRIMITIVES = new Map([
  ["src/components/ui/input.tsx", "Input"],
  ["src/components/ui/textarea.tsx", "Textarea"],
  ["src/components/ui/checkbox.tsx", "Checkbox: sr-only control"],
  ["src/components/ui/radio.tsx", "Radio: sr-only control"],
  ["src/components/ui/time-field.tsx", "TimeField: native time input"],
  ["src/components/ui/date-picker.tsx", "DatePicker: text entry of the date"],
  [
    "src/components/ui/color-swatch-input.tsx",
    "ColorSwatchInput: colour dialog",
  ],
  ["src/components/ui/page-header/SearchBar.tsx", "SearchBar: combobox field"],
  ["src/components/ui/listbox-dropdown.tsx", "ListboxDropdown: filter field"],
]);

const noRawInput = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Build form fields from the kit instead of a hand-styled <input> or <textarea>.",
    },
    messages: {
      rawTextarea:
        "Kein eigenes <textarea>. Textarea aus ~/components/ui/textarea nehmen; autoGrow wächst mit dem Inhalt (#5587, .claude/rules/frontend-ui-kit.md).",
      rawInput:
        'Kein eigenes <input>. Input, Checkbox, Radio, TimeField, ConsentCheckbox oder ColorSwatchInput aus ~/components/ui nehmen (#5217, .claude/rules/frontend-ui-kit.md). Nur type="hidden" und type="file" bleiben nativ.',
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    // Test mocks stand in for a kit control and render its native input.
    if (
      EXEMPT_FILE_RE.test(key) ||
      RAW_INPUT_KIT_PRIMITIVES.has(key) ||
      key.includes("src/test/")
    ) {
      return {};
    }
    return {
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name === "textarea") {
          context.report({ node, messageId: "rawTextarea" });
          return;
        }
        if (name !== "input") return;
        if (NATIVE_INPUT_TYPES.has(staticInputType(node) ?? "")) return;
        context.report({ node, messageId: "rawInput" });
      },
    };
  },
};

// --- ui-kit/no-hand-rolled-button + ui-kit/no-button-override (#5531) ------
//
// Jede Aktion nutzt Button, ButtonLink oder buttonClassName aus dem Kit.
//
//   no-hand-rolled-button — natives <button>, <a>, <Link>, <NavLink>,
//     <NavigationLink> oder ein Element mit role="button", das wie ein Knopf
//     aussieht: Hintergrund- oder Ring-/Rahmenfarbe plus Radius plus Polster
//     oder Höhe, jeweils ohne Zustandspräfix.
//   no-button-override — className an Button/ButtonLink oder in
//     buttonClassName({ className }) mit Maß- oder Optik-Klassen (Höhe,
//     Mindesthöhe, Polster, Radius, Schriftgröße/-gewicht, Hintergrund, Text-,
//     Ring- und Schattenfarbe, scale). Reine Layout-Klassen bleiben erlaubt.
//
// Ein Fund je Element. Die Zahl je Datei steht in ui-kit-button-baseline.json
// (Ist-Stand, shrink-only: mehr Funde schlagen hier fehl; dass die Baseline
// nach einem Abbau gesenkt wird, prüft oxlint-plugin-ui-kit.test.ts über
// `ui-kit-button-baseline-update.mjs --check`). Bewusste Ausnahmen stehen mit Begründung in der
// Allowlist in ui-kit-button-baseline.mjs. Neu erzeugen:
// `pnpm run ui-kit:button-baseline`.

const BUTTON_KIT_FILE = "src/components/ui/button.tsx";
const NATIVE_ACTION_ELEMENTS = new Set([
  "button",
  "a",
  "Link",
  "NavLink",
  "NavigationLink",
]);
const KIT_BUTTONS = new Set(["Button", "ButtonLink"]);
const KIT_BUTTON_EXPORTS = new Set(["Button", "ButtonLink", "buttonClassName"]);
const ACTION_COMPONENTS = new Set(["Link", "NavLink", "NavigationLink"]);
const DEFAULT_ACTION_COMPONENTS = new Map([
  ["next/link", "Link"],
  ["~/components/ui/nav-link", "NavLink"],
  ["~/components/ui/navigation-link", "NavigationLink"],
]);

// A colour value: a palette step (`gray-200`, `moto-red/20`), a named colour or
// an arbitrary colour. Positive on purpose: `border-b-2`, `border-l-4` and
// `bg-opacity-50` name a side, a width or an opacity, not a colour.
const COLOUR_VALUE = String.raw`(?:(?!opacity-)[a-z]+(?:-[a-z]+)*-\d{2,3}|moto-[\w-]+|white|black|current|\[(?:#|rgb|hsl|oklch|color)[^\]]*\])(?:\/[\w.[\]]+)?$`;
const BG_COLOR_RE = new RegExp(String.raw`^bg-${COLOUR_VALUE}`);
const FRAME_COLOR_RE = new RegExp(
  String.raw`^(?:ring|border(?:-[xytblrse])?)-${COLOUR_VALUE}`,
);
const RADIUS_RE = /^rounded(?:-(?!none$)[\w[\]./-]+)?$/;
const BOX_RE = /^(?:p|px|py|pt|pb|pl|pr|ps|pe|h|min-h|size)-/;

const OVERRIDE_RES = [
  /^-?(?:p|px|py|pt|pb|pl|pr|ps|pe|h|min-h|max-h|size)-/,
  /^rounded/,
  /^text-(?:xs|sm|base|lg|xl|\d+xl|\[\d)/,
  /^text-(?!(?:left|center|right|justify|start|end|wrap|nowrap|balance|pretty|ellipsis|clip)$)[a-z]/,
  /^font-(?:thin|extralight|light|normal|medium|semibold|bold|extrabold|black)$/,
  /^bg-(?!none$)/,
  /^ring(?:$|-)/,
  /^shadow(?:$|-)/,
  /^-?scale-/,
];

/** `md:hover:!bg-x` → `bg-x` */
function bareToken(token) {
  return token.replace(/^(?:[^\s:]+:)+/, "").replace(/^!/, "");
}

function unprefixedTokens(text) {
  return text.split(/\s+/).filter((token) => token && !token.includes(":"));
}

function hasButtonOptics(text) {
  const tokens = unprefixedTokens(text).map((token) => token.replace(/^!/, ""));
  const colour = tokens.some(
    (token) => BG_COLOR_RE.test(token) || FRAME_COLOR_RE.test(token),
  );
  return (
    colour &&
    tokens.some((token) => RADIUS_RE.test(token)) &&
    tokens.some((token) => BOX_RE.test(token))
  );
}

function overrideTokens(text) {
  return text
    .split(/\s+/)
    .filter(Boolean)
    .filter((token) => OVERRIDE_RES.some((re) => re.test(bareToken(token))));
}

function jsxAttribute(openingElement, name) {
  return openingElement.attributes.find(
    (attribute) =>
      attribute.type === "JSXAttribute" &&
      attribute.name.type === "JSXIdentifier" &&
      attribute.name.name === name,
  );
}

function staticAttributeValue(attribute) {
  const value = attribute?.value;
  if (value?.type === "Literal") return String(value.value);
  if (
    value?.type === "JSXExpressionContainer" &&
    value.expression.type === "Literal"
  ) {
    return String(value.expression.value);
  }
  return null;
}

/**
 * Static class text of an expression. Same-file constants (`const chip =
 * "…"`) are followed one level, so a class chain moved into a variable still
 * counts. `buttonClassName(…)` is the kit itself and contributes nothing here;
 * its className goes through no-button-override.
 */
function classText(node, constants) {
  const chunks = [];
  const visit = (current, depth) => {
    if (!current || typeof current !== "object") return;
    if (
      current.type === "CallExpression" &&
      current.callee?.type === "Identifier" &&
      current.callee.name === "buttonClassName"
    ) {
      return;
    }
    if (current.type === "Identifier" && depth < 2) {
      const init = constants.get(current.name);
      if (init) visit(init, depth + 1);
      return;
    }
    if (current.type === "Literal") {
      if (typeof current.value === "string") chunks.push(current.value);
      return;
    }
    if (current.type === "TemplateElement") {
      chunks.push(current.value.raw);
      return;
    }
    for (const [key, value] of Object.entries(current)) {
      if (key === "parent" || key === "loc" || key === "range") continue;
      if (Array.isArray(value)) {
        for (const child of value) visit(child, depth);
      } else if (value && typeof value === "object") {
        visit(value, depth);
      }
    }
  };
  visit(node, 0);
  return chunks.join(" ");
}

function classNameProperty(objectExpression) {
  if (objectExpression?.type !== "ObjectExpression") return null;
  return (
    objectExpression.properties.find(
      (property) =>
        property.type === "Property" &&
        ((property.key.type === "Identifier" &&
          property.key.name === "className") ||
          (property.key.type === "Literal" &&
            property.key.value === "className")),
    )?.value ?? null
  );
}

function buttonRatchetImports(node, bindings) {
  const source =
    node.source.type === "Literal" ? String(node.source.value) : "";
  const defaultAction = DEFAULT_ACTION_COMPONENTS.get(source);
  for (const specifier of node.specifiers) {
    if (specifier.type === "ImportDefaultSpecifier" && defaultAction) {
      bindings.actionComponents.add(specifier.local.name);
      if (NAVIGATION_ROWS.has(defaultAction)) {
        bindings.navigationRows.add(specifier.local.name);
      }
      continue;
    }
    if (specifier.type !== "ImportSpecifier") continue;
    const imported =
      specifier.imported.type === "Identifier"
        ? specifier.imported.name
        : String(specifier.imported.value);
    if (ACTION_COMPONENTS.has(imported)) {
      bindings.actionComponents.add(specifier.local.name);
    }
    if (NAVIGATION_ROWS.has(imported)) {
      bindings.navigationRows.add(specifier.local.name);
    }
    if (KIT_BUTTON_EXPORTS.has(imported)) {
      if (imported === "buttonClassName") {
        bindings.buttonClassNames.add(specifier.local.name);
      } else {
        bindings.kitButtons.add(specifier.local.name);
      }
    }
  }
}

function splitAllowlistedFindings(rule, key, findings, allowlist) {
  const allowances = new Map();
  for (const entry of allowlist) {
    if (entry.rule !== rule || entry.file !== key) continue;
    for (const line of entry.lines) {
      allowances.set(line, (allowances.get(line) ?? 0) + 1);
    }
  }
  const remaining = [];
  let allowlisted = 0;
  for (const node of findings) {
    const line = node.loc?.start.line;
    const count = allowances.get(line) ?? 0;
    if (count > 0) {
      allowances.set(line, count - 1);
      allowlisted += 1;
    } else {
      remaining.push(node);
    }
  }
  return { allowlisted, remaining };
}

/**
 * Collects findings per file and compares the total with allowlist plus
 * baseline at Program:exit. With UI_KIT_BUTTON_RATCHET=count every finding
 * is reported on its own; the baseline generator subtracts the allowlist.
 */
function makeRatchetRule({
  ruleName,
  description,
  message,
  detect,
  baselines = buttonRatchetBaseline,
  allowlist = buttonRatchetAllowlist,
  countEnv = "UI_KIT_BUTTON_RATCHET",
  kitFile = BUTTON_KIT_FILE,
  exemptDir = null,
  followStaticClassSources = false,
}) {
  return {
    meta: {
      type: "problem",
      docs: { description },
      messages: {
        finding: message,
        overBaseline: `${message} Diese Datei hat {{found}} Funde, erlaubt sind {{allowed}} (Allowlist {{allowlisted}} + Baseline {{baseline}}).`,
      },
      schema: [],
    },
    create(context) {
      const key = fileKey(context);
      if (
        EXEMPT_FILE_RE.test(key) ||
        key.endsWith(kitFile) ||
        (exemptDir !== null && key.startsWith(exemptDir)) ||
        key.includes("src/test/")
      ) {
        return {};
      }
      const constants = new Map();
      const bindings = {
        actionComponents: new Set(ACTION_COMPONENTS),
        buttonClassNames: new Set(["buttonClassName"]),
        kitButtons: new Set(KIT_BUTTONS),
        navigationRows: new Set(NAVIGATION_ROWS),
      };
      const findings = [];
      const countMode = process.env[countEnv] === "count";
      return {
        Program(node) {
          for (const statement of node.body) {
            if (statement.type === "ImportDeclaration") {
              buttonRatchetImports(statement, bindings);
            }
          }
          if (followStaticClassSources) {
            collectStaticClassSources(node, constants, context);
          }
        },
        VariableDeclarator(node) {
          if (node.id?.type === "Identifier" && node.init) {
            constants.set(node.id.name, node.init);
          }
        },
        ...detect({
          classText: (node) => classText(node, constants),
          found: (node) => findings.push(node),
          bindings,
          constants,
        }),
        "Program:exit"() {
          const { allowlisted, remaining } = splitAllowlistedFindings(
            ruleName,
            key,
            findings,
            allowlist,
          );
          const baseline = baselines[ruleName]?.[key] ?? 0;
          const allowed = allowlisted + baseline;
          if (countMode) {
            for (const node of findings) {
              context.report({ node, messageId: "finding" });
            }
            return;
          }
          if (remaining.length > baseline) {
            for (const node of remaining) {
              context.report({
                node,
                messageId: allowed === 0 ? "finding" : "overBaseline",
                data: {
                  found: String(remaining.length + allowlisted),
                  allowed: String(allowed),
                  allowlisted: String(allowlisted),
                  baseline: String(baseline),
                },
              });
            }
          }
        },
      };
    },
  };
}

const noHandRolledButton = makeRatchetRule({
  ruleName: "no-hand-rolled-button",
  description:
    "Build actions from Button, ButtonLink or buttonClassName instead of a hand-styled native element.",
  message:
    "Kein eigener Knopf: Button, ButtonLink oder buttonClassName aus ~/components/ui/button nehmen (#5531, .claude/rules/frontend-ui-kit.md).",
  detect: ({ classText: textOf, found, bindings }) => ({
    JSXOpeningElement(node) {
      const name = jsxName(node.name);
      const isAction =
        NATIVE_ACTION_ELEMENTS.has(name ?? "") ||
        bindings.actionComponents.has(name ?? "") ||
        staticAttributeValue(jsxAttribute(node, "role")) === "button";
      if (!isAction) return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      if (hasButtonOptics(textOf(className.value))) found(node);
    },
  }),
});

const noButtonOverride = makeRatchetRule({
  ruleName: "no-button-override",
  description:
    "Keep size and look of kit buttons in the kit; className carries layout only.",
  message:
    "Kein Maß oder Aussehen am Kit-Knopf: Größe und Variante wählen, className nur für Layout (w-full, flex-1, gap-*, mt-*, shrink-0, self-*, hidden md:flex …) (#5531, .claude/rules/frontend-ui-kit.md).",
  detect: ({ classText: textOf, found, bindings }) => ({
    JSXOpeningElement(node) {
      if (!bindings.kitButtons.has(jsxName(node.name) ?? "")) return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      if (overrideTokens(textOf(className.value)).length > 0) found(node);
    },
    CallExpression(node) {
      if (
        node.callee?.type !== "Identifier" ||
        !bindings.buttonClassNames.has(node.callee.name)
      ) {
        return;
      }
      const value = classNameProperty(node.arguments[0]);
      if (value && overrideTokens(textOf(value)).length > 0) found(node);
    },
  }),
});

// --- ui-kit/no-grey-button-fill (#5574) -------------------------------------
//
// Knöpfe sind weiß (outline, surface, ghost), schwarz (primary) oder farbig.
// Ein hellgrau gefüllter Ruhezustand (bg-gray-100 bis -300) passt in keine
// Rangstufe und stand oft in zweitem Grauton neben einem anderen. Die Regel
// prüft jedes Element, das no-hand-rolled-button oder no-button-override
// prüft, auch in Ternär-Zweigen und Konstanten derselben Datei. Zustände
// (hover:, active:, aria-pressed:) dürfen grau tönen. Keine Knöpfe in diesem
// Sinn (frontend-ui-kit.md, Buttons): Schalter, Reiter, Menü- und
// Auswahleinträge (role), Navigationszeilen (aria-current, NavLink,
// NavigationLink) und die Spur eines SegmentedControl. Hart null: keine
// Baseline; begründete Ausnahmen in der Allowlist (ui-kit-button-baseline.mjs).

const GREY_REST_FILL_RE = /^bg-gray-[123]00(?:\/[\w.[\]]+)?$/;
const NOT_BUTTON_ROLES = new Set([
  "switch",
  "tab",
  "menuitem",
  "menuitemradio",
  "menuitemcheckbox",
  "option",
  "radio",
]);
const NAVIGATION_ROWS = new Set(["NavLink", "NavigationLink"]);

function isNavigationOrControl(node, name, bindings) {
  const role = staticAttributeValue(jsxAttribute(node, "role"));
  if (role && NOT_BUTTON_ROLES.has(role)) return true;
  if (
    jsxAttribute(node, "aria-pressed") ||
    jsxAttribute(node, "aria-selected")
  ) {
    return true;
  }
  if (jsxAttribute(node, "aria-current")) return true;
  return bindings.navigationRows.has(name);
}

const INTERACTION_STATE_VARIANTS = new Set([
  "active",
  "autofill",
  "checked",
  "disabled",
  "enabled",
  "focus",
  "focus-visible",
  "focus-within",
  "hover",
  "indeterminate",
  "invalid",
  "open",
  "placeholder-shown",
  "read-only",
  "required",
  "target",
  "user-invalid",
  "user-valid",
  "valid",
  "visited",
]);

function isInteractionStateVariant(variant) {
  return (
    INTERACTION_STATE_VARIANTS.has(variant) ||
    /^(?:aria|data|group|has|peer)-/.test(variant)
  );
}

function hasGreyRestFill(text) {
  return text.split(/\s+/).some((token) => {
    const variants = token.split(":");
    const hasInteractionState = variants
      .slice(0, -1)
      .some(isInteractionStateVariant);
    return (
      token && !hasInteractionState && GREY_REST_FILL_RE.test(bareToken(token))
    );
  });
}

const noGreyButtonFill = makeRatchetRule({
  ruleName: "no-grey-button-fill",
  description:
    "Disallow a light-grey resting fill on buttons; neutral actions are outline, surface or ghost.",
  message:
    "Kein hellgrauer Knopf: neutrale Aktionen sind outline, surface oder ghost, die Hauptaktion primary, Löschen outline_danger oder danger (#5574, .claude/rules/frontend-ui-kit.md).",
  detect: ({ classText: textOf, found, bindings }) => ({
    JSXOpeningElement(node) {
      const name = jsxName(node.name) ?? "";
      const isAction =
        NATIVE_ACTION_ELEMENTS.has(name) ||
        bindings.actionComponents.has(name) ||
        bindings.kitButtons.has(name) ||
        staticAttributeValue(jsxAttribute(node, "role")) === "button";
      if (!isAction || isNavigationOrControl(node, name, bindings)) return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      if (hasGreyRestFill(textOf(className.value))) found(node);
    },
    CallExpression(node) {
      if (
        node.callee?.type !== "Identifier" ||
        !bindings.buttonClassNames.has(node.callee.name)
      ) {
        return;
      }
      const value = classNameProperty(node.arguments[0]);
      if (value && hasGreyRestFill(textOf(value))) found(node);
    },
  }),
});

// --- Pillen (#5584) -----------------------------------------------------------

const PILL_KIT_FILE = "src/components/ui/status-badge.tsx";
const PILL_RADIUS_RE = /^rounded-(?:full|md)$/;
const PILL_TEXT_RE = /^text-(?:xs|\[10px\]|\[11px\])$/;
const DOT_SIZE_RE = /^(?:h|w|size)-(?:1|1\.5|2|2\.5)$/;
const HEX_ALPHA_RE = /^[0-9a-fA-F]{2}$/;
const BACKGROUND_KEYS = new Set(["background", "backgroundColor"]);

function pillTokens(text) {
  return unprefixedTokens(text).map((token) => token.replace(/^!/, ""));
}

function isPillShape(text) {
  const tokens = pillTokens(text);
  return (
    tokens.some((token) => PILL_RADIUS_RE.test(token)) &&
    tokens.some((token) => token.startsWith("px-")) &&
    tokens.some((token) => token.startsWith("py-")) &&
    tokens.some((token) => PILL_TEXT_RE.test(token))
  );
}

function isDot(text) {
  const tokens = pillTokens(text);
  return (
    tokens.includes("rounded-full") &&
    (tokens.some((token) => /^size-/.test(token) && DOT_SIZE_RE.test(token)) ||
      (tokens.some(
        (token) => token.startsWith("h-") && DOT_SIZE_RE.test(token),
      ) &&
        tokens.some(
          (token) => token.startsWith("w-") && DOT_SIZE_RE.test(token),
        )))
  );
}

function propertyName(property) {
  if (property.type !== "Property") return null;
  if (property.key.type === "Identifier") return property.key.name;
  if (property.key.type === "Literal") return String(property.key.value);
  return null;
}

function styleBackgroundValues(openingElement, constants) {
  const value = jsxAttribute(openingElement, "style")?.value;
  if (value?.type !== "JSXExpressionContainer") return [];
  let object = value.expression;
  if (object.type === "Identifier") object = constants.get(object.name);
  if (object?.type !== "ObjectExpression") return [];
  return object.properties
    .filter((property) => BACKGROUND_KEYS.has(propertyName(property) ?? ""))
    .map((property) => property.value);
}

function hasHexAlphaTemplate(node) {
  if (!node || typeof node !== "object") return false;
  if (node.type === "TemplateLiteral") {
    return node.quasis.some(
      (quasi, index) => index > 0 && HEX_ALPHA_RE.test(quasi.value.raw.trim()),
    );
  }
  if (node.type === "ConditionalExpression") {
    return (
      hasHexAlphaTemplate(node.consequent) ||
      hasHexAlphaTemplate(node.alternate)
    );
  }
  if (node.type === "LogicalExpression") {
    return hasHexAlphaTemplate(node.left) || hasHexAlphaTemplate(node.right);
  }
  return false;
}

// A hint box and a pill share their classes on a `div`. Only a pill sizes to
// its label: fully round, or inline. A blur marks a glass surface such as the
// brand chips in auth-shell, not a status.
const DIV_PILL_INLINE_RE = /^(?:inline-flex|inline-block)$/;
const GLASS_SURFACE_RE = /^backdrop-blur(?:-|$)/;

function isDivPill(text) {
  const tokens = pillTokens(text);
  return (
    (tokens.includes("rounded-full") ||
      tokens.some((token) => DIV_PILL_INLINE_RE.test(token))) &&
    !tokens.some((token) => GLASS_SURFACE_RE.test(token))
  );
}

function pillClassText(node, textOf) {
  const name = jsxName(node.name);
  if (name !== "span" && name !== "div") return null;
  if (staticAttributeValue(jsxAttribute(node, "role")) === "tooltip")
    return null;
  const className = jsxAttribute(node, "className");
  if (!className?.value) return null;
  const text = textOf(className.value);
  if (!isPillShape(text)) return null;
  return name === "span" || isDivPill(text) ? text : null;
}

// Static sources of a pill's classes beyond a `const` above the element:
// declarations further down the file and exported helpers or lookup tables of
// another module (`getDeviceStatusColor(status)`). A helper's own strings count;
// a table it reads from elsewhere in its module does not.
const CLASS_MODULE_EXTENSIONS = [".ts", ".tsx", "/index.ts", "/index.tsx"];
const classModuleCache = new Map();

function readClassModule(path) {
  if (!classModuleCache.has(path)) {
    let source = null;
    for (const extension of CLASS_MODULE_EXTENSIONS) {
      try {
        source = readFileSync(path + extension, "utf8");
        break;
      } catch {
        // Next candidate; a module outside the project stays unresolved.
      }
    }
    classModuleCache.set(path, source);
  }
  return classModuleCache.get(path);
}

function resolveClassModule(specifier, filePath) {
  if (specifier.startsWith("~/") || specifier.startsWith("@/")) {
    const root = filePath.lastIndexOf("/src/");
    return root === -1
      ? null
      : `${filePath.slice(0, root)}/src/${specifier.slice(2)}`;
  }
  return specifier.startsWith(".")
    ? normalize(join(dirname(filePath), specifier))
    : null;
}

function exportedStrings(source, name) {
  const start = new RegExp(
    String.raw`^export (?:async )?(?:function|const) ${name}\b`,
    "m",
  ).exec(source);
  if (!start) return null;
  const rest = source.slice(start.index + start[0].length);
  // Formatted code: the declaration ends at the next line in column 0.
  const end = rest.search(/\n\S/);
  const declaration = end === -1 ? rest : rest.slice(0, end);
  return (declaration.match(/"[^"\n]*"|'[^'\n]*'|`[^`]*`/g) ?? [])
    .join(" ")
    .replaceAll(/["'`]/g, " ");
}

function collectStaticClassSources(program, constants, context) {
  const filePath = String(
    context.physicalFilename ?? context.filename ?? "",
  ).replaceAll("\\", "/");
  for (const statement of program.body) {
    const declaration =
      statement.type === "ExportNamedDeclaration"
        ? statement.declaration
        : statement;
    if (declaration?.type === "FunctionDeclaration" && declaration.id) {
      constants.set(declaration.id.name, declaration);
    }
    if (declaration?.type === "VariableDeclaration") {
      for (const { id, init } of declaration.declarations) {
        if (id.type === "Identifier" && init) constants.set(id.name, init);
      }
    }
    if (statement.type !== "ImportDeclaration") continue;
    const path = resolveClassModule(String(statement.source.value), filePath);
    const source = path === null ? null : readClassModule(path);
    if (source === null) continue;
    for (const specifier of statement.specifiers) {
      if (specifier.type !== "ImportSpecifier") continue;
      const imported = specifier.imported.name ?? specifier.imported.value;
      const value = exportedStrings(source, imported);
      if (value) {
        constants.set(specifier.local.name, { type: "Literal", value });
      }
    }
  }
}

// Hard-zero: der Bestand ist abgebaut, jede neue Stelle schlägt fehl.
function makePillRule(options) {
  return makeRatchetRule({
    ...options,
    baselines: {},
    allowlist: [],
    kitFile: PILL_KIT_FILE,
    followStaticClassSources: true,
  });
}

const noHandRolledPill = makePillRule({
  ruleName: "no-hand-rolled-pill",
  description:
    "Build status pills from StatusBadge, StatusColorBadge or DataTableStatusBadge instead of a hand-styled span or div.",
  message:
    "Keine eigene Pille: StatusBadge (Töne blue, green, orange, red, gray, purple, cyan, outline; size xs für dichte Raster) oder StatusColorBadge (Farbe aus LOCATION_COLORS, amber über LOCATION_COLORS.WARNING) nehmen (#5584, .claude/rules/frontend-ui-kit.md).",
  detect: ({ classText: textOf, found, constants }) => ({
    JSXOpeningElement(node) {
      const text = pillClassText(node, textOf);
      if (text === null) return;
      const surface =
        pillTokens(text).some((token) => BG_COLOR_RE.test(token)) ||
        styleBackgroundValues(node, constants).length > 0;
      if (surface) found(node);
    },
  }),
});

const noPillDot = makePillRule({
  ruleName: "no-pill-dot",
  description: "Disallow a decorative dot inside a status pill (#2476).",
  message:
    "Kein Punkt in einer Pille: Tönung und Text nennen den Status schon (#2476, #5584). Punkte nur über StatusDot in Legenden und Plänen.",
  detect: ({ classText: textOf, found }) => ({
    JSXOpeningElement(node) {
      if (pillClassText(node, textOf) === null) return;
      for (const child of node.parent?.children ?? []) {
        if (child.type !== "JSXElement") continue;
        const className = jsxAttribute(child.openingElement, "className");
        if (className?.value && isDot(textOf(className.value))) {
          found(child.openingElement);
        }
      }
    },
  }),
});

// --- Punkte und Farbbalken (#5598, ADR 0149 Entscheidung 5) --------------------

const HAND_DOT_RE = /^(?:h|w|size)-(?:1|1\.5|2|2\.5|3)$/;
const COLOR_BAR_HEIGHT_RE = /^h-(?:3|3\.5|9)$/;
const COLOR_BAR_WIDTH_RE = /^w-(?:0\.5|1)$/;
const DOT_NOT_A_FILL_RE = /^(?:animate-spin|border(?:-.*)?)$/;

function hasDotShape(tokens) {
  if (!tokens.includes("rounded-full")) return false;
  const size = (prefix) =>
    tokens.find((token) => token.startsWith(prefix) && HAND_DOT_RE.test(token));
  if (size("size-")) return true;
  const height = size("h-");
  const width = size("w-");
  if (height && width && height.slice(2) === width.slice(2)) return true;
  return (
    tokens.some((token) => COLOR_BAR_HEIGHT_RE.test(token)) &&
    tokens.some((token) => COLOR_BAR_WIDTH_RE.test(token))
  );
}

function classHasExpression(value) {
  if (value?.type !== "JSXExpressionContainer") return false;
  const { expression } = value;
  return (
    expression.type !== "Literal" &&
    !(
      expression.type === "TemplateLiteral" &&
      expression.expressions.length === 0
    )
  );
}

const noHandRolledDot = makeRatchetRule({
  ruleName: "no-hand-rolled-dot",
  description:
    "Build dots and colour bars from StatusDot, ColorBar or PlanLegend instead of a hand-styled element.",
  message:
    "Kein Punkt und kein Farbbalken von Hand: in Legenden und Plänen StatusDot, ColorBar oder PlanLegend; sonst nennt StatusBadge den Status mit Text (#5598, .claude/rules/frontend-ui-kit.md).",
  // Hard-zero: der Bestand ist abgebaut, jede neue Stelle schlägt fehl.
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/status-dot.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ classText: textOf, found, constants }) => ({
    JSXOpeningElement(node) {
      const name = jsxName(node.name);
      if (name !== "span" && name !== "div") return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      const text = textOf(className.value);
      const tokens = pillTokens(text);
      if (!hasDotShape(tokens)) return;
      if (tokens.some((token) => DOT_NOT_A_FILL_RE.test(token))) return;
      const filled =
        tokens.some((token) => BG_COLOR_RE.test(token)) ||
        styleBackgroundValues(node, constants).length > 0 ||
        classHasExpression(className.value);
      if (!filled) return;
      // Ein Punkt in einer Pille zählt no-pill-dot.
      const parent = node.parent?.parent;
      if (
        parent?.type === "JSXElement" &&
        pillClassText(parent.openingElement, textOf) !== null
      ) {
        return;
      }
      found(node);
    },
  }),
});

// Pläne und Legenden, in denen heute schon Punkte stehen. Neue Fläche nur mit
// Freigabe: StatusDot ist kein allgemeines Status-Mittel.
const STATUS_DOT_SCOPE = [
  UI_KIT_DIR,
  "src/components/timetable/",
  "src/components/calendar/",
  "src/components/parent/calendar/",
  "src/components/staff/dienstplan-",
  "src/components/staff/uebersicht-tab",
  "src/components/time-tracking/vacation-request-modal",
  "src/components/home/home-block-content",
];

const noStatusDotOutsidePlans = {
  meta: {
    type: "problem",
    docs: { description: "StatusDot only in legends and plans (ADR 0149)." },
    messages: {
      outside:
        "StatusDot nur in Legenden und Plänen, wo heute schon Punkte stehen (ADR 0149, Entscheidung 5). Einen Status mit Text zeigt StatusBadge.",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (
      EXEMPT_FILE_RE.test(key) ||
      STATUS_DOT_SCOPE.some((scope) => key.startsWith(scope))
    ) {
      return {};
    }
    return {
      ImportDeclaration(node) {
        if (/(?:^|\/)ui\/status-dot$/.test(String(node.source.value))) {
          context.report({ node, messageId: "outside" });
        }
      },
    };
  },
};

const noHexAlphaTint = makePillRule({
  ruleName: "no-hex-alpha-tint",
  description:
    "Disallow `${color}1A`-style hex alpha tints in a pill's style; the kit tone carries the tint.",
  message:
    "Keine Tönung per Hex-Alpha (`${…}1A`) im style einer Pille: StatusColorBadge color={…} tönt mit lesbarem Text (#5584).",
  detect: ({ classText: textOf, found, constants }) => ({
    JSXOpeningElement(node) {
      if (pillClassText(node, textOf) === null) return;
      if (styleBackgroundValues(node, constants).some(hasHexAlphaTemplate)) {
        found(node);
      }
    },
  }),
});

// --- ui-kit/no-hand-rolled-icon-tile (#5582) --------------------------------
//
// Eine Icon-Kachel (zentriertes Symbol auf grauer oder getönter Fläche) kommt
// aus `IconTile` / `ConceptIconTile`. Erkannt wird die Klassenkette
// `flex h-(8–12) w-(8–12)` (oder `size-(8–12)`) `items-center justify-center
// rounded-(lg|xl|2xl|full) bg-(gray|moto)-…` an einem Element, dessen Kind ein Symbol ist. Kreise mit
// Initialen oder Schrittnummern (Text als Kind) und Schaltflächen bleiben frei.

const ICON_TILE_FILE = "src/components/ui/icon-tile.tsx";
// `size-10` steht für `h-10 w-10`; `inline-flex` baut dieselbe Kachel.
const ICON_TILE_BOX_RE =
  /(?:^|\s)(?:size-(?:8|9|10|11|12)|h-(?:8|9|10|11|12)\s(?:\S+\s)*?w-(?:8|9|10|11|12)|w-(?:8|9|10|11|12)\s(?:\S+\s)*?h-(?:8|9|10|11|12))(?=\s|$)/;
const ICON_TILE_TOKENS = [
  /(?:^|\s)(?:inline-)?flex(?=\s|$)/,
  ICON_TILE_BOX_RE,
  /(?:^|\s)items-center(?=\s|$)/,
  /(?:^|\s)justify-center(?=\s|$)/,
  /(?:^|\s)rounded-(?:lg|xl|2xl|full)(?=\s|$)/,
  /(?:^|\s)bg-(?:gray|moto)-\S+/,
];
const INTERACTIVE_ELEMENTS = new Set([
  "button",
  "a",
  "Link",
  "NavLink",
  "NavigationLink",
  "Button",
  "ButtonLink",
  "label",
]);

const isIconTileCombo = (text) =>
  ICON_TILE_TOKENS.every((token) => token.test(text));

function expressionNamesIcon(expression) {
  if (!expression) return false;
  if (expression.type === "Identifier") return /icon/i.test(expression.name);
  if (expression.type === "MemberExpression") {
    return (
      expression.property?.type === "Identifier" &&
      /icon/i.test(expression.property.name)
    );
  }
  if (expression.type === "JSXElement")
    return elementNamesIcon(expression) || hasIconChild(expression);
  if (expression.type === "JSXFragment") return hasIconChild(expression);
  if (expression.type === "ConditionalExpression") {
    return (
      expressionNamesIcon(expression.consequent) ||
      expressionNamesIcon(expression.alternate)
    );
  }
  if (expression.type === "LogicalExpression") {
    return expressionNamesIcon(expression.right);
  }
  return false;
}

function elementNamesIcon(element) {
  const name = element.openingElement?.name;
  if (name?.type !== "JSXIdentifier") return false;
  if (name.name === "svg" || /icon/i.test(name.name)) return true;
  return /^[A-Z]/.test(name.name) && (element.children?.length ?? 0) === 0;
}

/** Ein Symbol als Kind, kein Text: Initialen und Schrittnummern sind keine Kachel. */
function hasIconChild(element) {
  let icon = false;
  for (const child of element.children ?? []) {
    if (child.type === "JSXText") {
      if (child.value.trim() !== "") return false;
      continue;
    }
    if (child.type === "JSXElement") {
      if (elementNamesIcon(child)) icon = true;
      else if (hasIconChild(child)) icon = true;
      else return false;
      continue;
    }
    if (child.type === "JSXExpressionContainer") {
      if (child.expression.type === "JSXEmptyExpression") continue;
      if (expressionNamesIcon(child.expression)) icon = true;
      else return false;
    }
  }
  return icon;
}

const noHandRolledIconTile = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-rolled icon tiles; use IconTile or ConceptIconTile from the kit.",
    },
    messages: {
      handRolledIconTile:
        "Handgebaute Icon-Kachel. IconTile (~/components/ui/icon-tile) mit size, shape und tone nehmen, für ein fachliches Konzept ConceptIconTile (#5582, .claude/rules/frontend-ui-kit.md).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key) || key === ICON_TILE_FILE) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode?.();

    return {
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name === null || INTERACTIVE_ELEMENTS.has(name)) return;
        const classAttribute = node.attributes.find(
          (attribute) =>
            attribute.type === "JSXAttribute" &&
            attribute.name.type === "JSXIdentifier" &&
            attribute.name.name === "className",
        );
        if (!classAttribute?.value) return;
        const combos = classCombinations(
          classAttribute.value,
          new WeakSet(),
          sourceCode,
        ) ?? [
          (() => {
            const chunks = [];
            collectStaticStrings(classAttribute.value, chunks);
            return chunks.join(" ");
          })(),
        ];
        if (!combos.some(isIconTileCombo)) return;
        if (!hasIconChild(node.parent)) return;
        context.report({ node, messageId: "handRolledIconTile" });
      },
    };
  },
};

// --- ui-kit/no-select-override (#5580) --------------------------------------
//
// Auswahlfelder bringen ihre Optik aus dem Kit mit: ListboxDropdown zeichnet
// Auslöser, Menü, Option und Kästchen, CustomSelect und MultiSelect setzen
// darauf auf. Ein Aufrufer wählt `controlSize`/`size` und gibt nur Layout
// (Breite, flex, Schriftgewicht) in className/menuClassName. Rahmen-,
// Hintergrund-, Höhen- und Radiusklassen sowie die Flächen-Utilities sind
// dort verboten, ebenso `unstyledTrigger` außerhalb des Kits. Hart null.

const SELECT_COMPONENT_MODULES = new Map([
  ["~/components/ui/custom-select", "CustomSelect"],
  ["@/components/ui/custom-select", "CustomSelect"],
  ["~/components/ui/multi-select", "MultiSelect"],
  ["@/components/ui/multi-select", "MultiSelect"],
  ["~/components/ui/listbox-dropdown", "ListboxDropdown"],
  ["@/components/ui/listbox-dropdown", "ListboxDropdown"],
  ["~/components/ui/date-range-picker", "DateRangePicker"],
  ["@/components/ui/date-range-picker", "DateRangePicker"],
]);
const SELECT_COMPONENTS = new Set([
  "CustomSelect",
  "MultiSelect",
  "ListboxDropdown",
  // Sein Auslöser ist der der Auswahlfelder (#5585).
  "DateRangePicker",
]);
const SELECT_CLASS_PROPS = [
  "className",
  "triggerClassName",
  "menuClassName",
  "containerClassName",
];
const SELECT_OVERRIDE_RES = [
  /^border(?:-|$)/,
  /^bg-/,
  /^(?:min-|max-)?h-/,
  /^size-/,
  /^rounded(?:-|$)/,
  /^moto-(?:content|popover)-surface$/,
];

function selectOverrideTokens(text) {
  return text
    .split(/\s+/)
    .filter(Boolean)
    .filter((token) =>
      SELECT_OVERRIDE_RES.some((re) => re.test(bareToken(token))),
    );
}

const noSelectOverride = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Keep frame, fill, height and radius of select fields in the kit; class props carry layout only.",
    },
    messages: {
      override:
        'Keine eigene Optik am Auswahlfeld ({{tokens}}): Rahmen, Hintergrund, Höhe und Radius kommen aus dem Kit. controlSize="compact" (ListboxDropdown: size) wählen, className nur für Layout (#5580, .claude/rules/frontend-ui-kit.md).',
      unstyled:
        "unstyledTrigger ist nur im Kit erlaubt; außerhalb gilt die Feldoptik von ListboxDropdown (#5580).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (EXEMPT_FILE_RE.test(key) || key.includes(UI_KIT_DIR)) return {};
    const constants = new Map();
    const selects = new Set();
    const pending = [];
    return {
      ImportDeclaration(node) {
        const source =
          node.source.type === "Literal" ? String(node.source.value) : "";
        const exported = SELECT_COMPONENT_MODULES.get(source);
        if (!exported) return;
        for (const specifier of node.specifiers) {
          if (specifier.type !== "ImportSpecifier") continue;
          const imported =
            specifier.imported.type === "Identifier"
              ? specifier.imported.name
              : String(specifier.imported.value);
          if (SELECT_COMPONENTS.has(imported)) {
            selects.add(specifier.local.name);
          }
        }
      },
      VariableDeclarator(node) {
        if (node.id?.type === "Identifier" && node.init) {
          constants.set(node.id.name, node.init);
        }
      },
      JSXOpeningElement(node) {
        if (!selects.has(jsxName(node.name) ?? "")) return;
        pending.push(node);
      },
      "Program:exit"() {
        for (const node of pending) {
          if (jsxAttribute(node, "unstyledTrigger")) {
            context.report({ node, messageId: "unstyled" });
          }
          const tokens = SELECT_CLASS_PROPS.flatMap((name) => {
            const attribute = jsxAttribute(node, name);
            if (!attribute?.value) return [];
            return selectOverrideTokens(classText(attribute.value, constants));
          });
          if (tokens.length > 0) {
            context.report({
              node,
              messageId: "override",
              data: { tokens: [...new Set(tokens)].join(" ") },
            });
          }
        }
      },
    };
  },
};

// --- ui-kit/no-control-height-override (#5605) -----------------------------
//
// Spec R17: Es gibt eine Bedienhöhe. Input wählt sie über `controlSize`, die
// Auswahlfelder ebenso, OverflowMenu über `triggerSize`. `h-8`, `h-9` und
// `!size-*` an diesen Bauteilen stellten 36- und 28-px-Felder neben die
// 40er. Hart null.

const CONTROL_HEIGHT_MODULES = new Map([
  ["~/components/ui/input", "Input"],
  ["@/components/ui/input", "Input"],
  ["~/components/ui/custom-select", "CustomSelect"],
  ["@/components/ui/custom-select", "CustomSelect"],
  ["~/components/ui/multi-select", "MultiSelect"],
  ["@/components/ui/multi-select", "MultiSelect"],
  ["~/components/ui/date-range-picker", "DateRangePicker"],
  ["@/components/ui/date-range-picker", "DateRangePicker"],
  ["~/components/ui/page-header/OverflowMenu", "OverflowMenu"],
  ["@/components/ui/page-header/OverflowMenu", "OverflowMenu"],
]);
const CONTROL_HEIGHT_CLASS_PROPS = [
  "className",
  "triggerClassName",
  "containerClassName",
];

function controlHeightTokens(text) {
  return text
    .split(/\s+/)
    .filter(Boolean)
    .filter((token) => {
      const utility = token.replace(/^(?:[^\s:]+:)+/, "");
      return /^!?h-[89]$/.test(utility) || /^!size-/.test(utility);
    });
}

const noControlHeightOverride = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Keep the height of inputs, selects and overflow menus in the kit.",
    },
    messages: {
      override:
        'Keine eigene Bedienhöhe ({{tokens}}): Input und Auswahlfelder nehmen controlSize="compact" (40 px), OverflowMenu triggerSize (#5605, .claude/rules/frontend-ui-kit.md).',
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (EXEMPT_FILE_RE.test(key) || key.includes(UI_KIT_DIR)) return {};
    const constants = new Map();
    const controls = new Set();
    const pending = [];
    return {
      ImportDeclaration(node) {
        const source =
          node.source.type === "Literal" ? String(node.source.value) : "";
        const exported = CONTROL_HEIGHT_MODULES.get(source);
        if (!exported) return;
        for (const specifier of node.specifiers) {
          if (specifier.type !== "ImportSpecifier") continue;
          const imported =
            specifier.imported.type === "Identifier"
              ? specifier.imported.name
              : String(specifier.imported.value);
          if (imported === exported) controls.add(specifier.local.name);
        }
      },
      VariableDeclarator(node) {
        if (node.id?.type === "Identifier" && node.init) {
          constants.set(node.id.name, node.init);
        }
      },
      JSXOpeningElement(node) {
        if (controls.has(jsxName(node.name) ?? "")) pending.push(node);
      },
      "Program:exit"() {
        for (const node of pending) {
          const tokens = CONTROL_HEIGHT_CLASS_PROPS.flatMap((name) => {
            const attribute = jsxAttribute(node, name);
            if (!attribute?.value) return [];
            return controlHeightTokens(classText(attribute.value, constants));
          });
          if (tokens.length > 0) {
            context.report({
              node,
              messageId: "override",
              data: { tokens: [...new Set(tokens)].join(" ") },
            });
          }
        }
      },
    };
  },
};

// --- ui-kit/no-important-override + ui-kit/no-section-card-bleed (#5581) ---
//
// Fehlt einer Kit-Komponente eine Prop, wird sie im Kit ergänzt, nicht von
// außen mit `!` überstimmt (`<SectionCard className="!p-0">`). Die Regel
// prüft jedes className-Attribut (auch bodyClassName, titleClassName …) an
// Elementen, die aus ~/components/ui/ importiert sind. Button und ButtonLink
// gehören der Button-Ratsche (no-button-override) mit ihrer Baseline.
//
// no-section-card-bleed fällt auf einen Negativrand im bodyClassName einer
// SectionCard: `-mx-5 -mb-5` hob das Kartenpolster für einen randlosen Rumpf
// wieder auf und stimmte auf dem Telefon (16 px Polster) schon nicht mehr.
// Dafür gibt es `flush` und `footer`. Hart null.
//
// no-important-override startet mit einer shrink-only Baseline je Datei
// (ui-kit-important-override-baseline.mjs).

const KIT_IMPORT_RE = /^(?:~|@)\/components\/ui\//;
const IMPORTANT_OWNED_ELSEWHERE = new Set(["Button", "ButtonLink"]);
const NEGATIVE_MARGIN_RE = /^-m[xytrblse]?-/;

function kitComponentNames(program) {
  const names = new Set();
  for (const statement of program.body) {
    if (statement.type !== "ImportDeclaration") continue;
    const source =
      statement.source.type === "Literal" ? String(statement.source.value) : "";
    if (!KIT_IMPORT_RE.test(source)) continue;
    for (const specifier of statement.specifiers) {
      if (specifier.type === "ImportNamespaceSpecifier") continue;
      const imported =
        specifier.type === "ImportSpecifier"
          ? specifier.imported.type === "Identifier"
            ? specifier.imported.name
            : String(specifier.imported.value)
          : "default";
      if (IMPORTANT_OWNED_ELSEWHERE.has(imported)) continue;
      names.add(specifier.local.name);
    }
  }
  return names;
}

/** Utility without its variant prefixes (`md:hover:!p-0` → `!p-0`). */
function bareUtility(token) {
  let depth = 0;
  let start = 0;
  for (let index = 0; index < token.length; index += 1) {
    const char = token[index];
    if (char === "[") depth += 1;
    else if (char === "]") depth -= 1;
    else if (char === ":" && depth === 0) start = index + 1;
  }
  return token.slice(start);
}

function isImportantUtility(token) {
  const utility = bareUtility(token);
  if (utility.length < 2) return false;
  return (
    (utility.startsWith("!") && /^[-a-z[]/.test(utility.slice(1))) ||
    (utility.endsWith("!") && /^-?[a-z]/.test(utility))
  );
}

function classAttributes(node) {
  return node.attributes.filter(
    (attribute) =>
      attribute.type === "JSXAttribute" &&
      attribute.name.type === "JSXIdentifier" &&
      /^(?:className|[a-z]\w*ClassName)$/.test(attribute.name.name) &&
      attribute.value,
  );
}

function makeKitClassRule({
  description,
  messageId,
  message,
  detect,
  baseline = {},
}) {
  return {
    meta: {
      type: "problem",
      docs: { description },
      messages: { [messageId]: message },
      schema: [],
    },
    create(context) {
      const key = fileKey(context);
      if (EXEMPT_FILE_RE.test(key) || key.includes("src/test/")) return {};
      const constants = new Map();
      const findings = [];
      let kitNames = new Set();
      return {
        Program(node) {
          kitNames = kitComponentNames(node);
        },
        "Program:exit"() {
          // count: jeden Fund melden, damit der Test die Baseline gegen den
          // Ist-Stand prüfen kann (shrink-only, wie bei der Button-Ratsche).
          const countMode = process.env.UI_KIT_IMPORTANT_OVERRIDE === "count";
          if (!countMode && findings.length <= (baseline[key] ?? 0)) return;
          for (const node of findings) context.report({ node, messageId });
        },
        VariableDeclarator(node) {
          if (node.id?.type === "Identifier" && node.init) {
            constants.set(node.id.name, node.init);
          }
        },
        JSXOpeningElement(node) {
          const name = jsxName(node.name);
          if (!name || !kitNames.has(name)) return;
          for (const attribute of classAttributes(node)) {
            const tokens = classText(attribute.value, constants)
              .split(/\s+/)
              .filter(Boolean);
            if (detect({ name, attribute: attribute.name.name, tokens })) {
              findings.push(attribute);
            }
          }
        },
      };
    },
  };
}

const noImportantOverride = makeKitClassRule({
  description:
    "Do not override a kit component with important utilities; add the missing prop to the kit.",
  messageId: "important",
  message:
    'Kein `!` an einer Kit-Komponente: die fehlende Prop im Kit ergänzen (SectionCard: padding="none", flush, footer) statt sie von außen zu überstimmen (#5581, .claude/rules/frontend-ui-kit.md).',
  detect: ({ tokens }) => tokens.some(isImportantUtility),
  baseline: importantOverrideBaseline,
});

const noSectionCardBleed = makeKitClassRule({
  description:
    "Do not undo the SectionCard padding with negative margins; use flush or footer.",
  messageId: "bleed",
  message:
    "Kein Negativrand im Rumpf einer SectionCard: `flush` lässt den Rumpf bis an die Kante laufen, `footer` ergibt eine randlose Fußzone (#5581).",
  detect: ({ name, attribute, tokens }) =>
    name === "SectionCard" &&
    attribute === "bodyClassName" &&
    tokens.some((token) => NEGATIVE_MARGIN_RE.test(bareUtility(token))),
});

// ui-kit/no-shadow-kit-name — eine lokale Funktion oder Konstante trägt nicht den
// Namen eines Kit-Exports (#5583). Wer in einer Datei `StatCard` liest, soll die
// Kit-Kachel vor sich haben, nicht einen Eigenbau, der nur so heißt. Die Namen
// kommen aus den Kit-Dateien selbst, damit die Liste nicht veraltet.
const KIT_EXPORT_RE =
  /^export\s+(?:default\s+)?(?:async\s+)?(?:function\*?|const)\s+([A-Za-z_$][\w$]*)|^export\s+default\s+([A-Za-z_$][\w$]*)\s*;|^export\s*\{([^}]*)\}/gm;

function readKitExportNames() {
  const kitDir = join(
    dirname(fileURLToPath(import.meta.url)),
    "../src/components/ui",
  );
  const names = new Set();
  const files = readdirSync(kitDir, { recursive: true, encoding: "utf8" });
  for (const file of files) {
    if (!/\.tsx?$/.test(file) || /\.(test|stories)\./.test(file)) continue;
    const source = readFileSync(join(kitDir, file), "utf8");
    for (const [, declared, byDefault, list] of source.matchAll(
      KIT_EXPORT_RE,
    )) {
      if (declared) names.add(declared);
      if (byDefault) names.add(byDefault);
      for (const entry of list?.split(",") ?? []) {
        if (/^\s*type\s/.test(entry)) continue;
        const exported = entry
          .trim()
          .split(/\s+as\s+/)
          .pop();
        if (exported) names.add(exported);
      }
    }
  }
  return names;
}

// Die Klassen, an denen sich ein Nachbau der Portal-Navigation verrät (#5603).
// Jede steht nur in portal-bottom-nav.tsx oder portal-sidebar.tsx.
const PORTAL_NAV_SHELL_SIGNATURES = [
  ["shadow-[0_-2px_20px_rgba(0,0,0,0.08)]"],
  ["absolute", "top-0", "h-full", "rounded-full", "bg-gray-100"],
  ["flex", "min-h-[44px]", "justify-center", "gap-2.5", "rounded-full"],
  ["border-r", "border-gray-200/70", "bg-white/95"],
  ["top-[73px]", "h-[calc(100vh-73px)]"],
  ["top-[121px]", "h-[calc(100vh-121px)]"],
  ["top-[153px]", "h-[calc(100vh-153px)]"],
];
const PORTAL_NAV_KIT_ROW_EXPORTS = new Set([
  "PORTAL_ICON_CLASSES",
  "portalSidebarRowClasses",
]);

function hasPortalNavShellSignature(text) {
  const tokens = new Set(text.split(/\s+/));
  return PORTAL_NAV_SHELL_SIGNATURES.some((signature) =>
    signature.every((token) => tokens.has(token)),
  );
}

const noHandRolledPortalNav = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Sidebar and bottom bar of a portal exist once, in the UI kit (#5603).",
    },
    messages: {
      shell:
        "Keine eigene Portal-Navigation: PortalSidebar, PortalSidebarFrame oder PortalBottomNav aus dem Kit nehmen (#5603, .claude/rules/frontend-ui-kit.md).",
      row: "`{{name}}` gehört in die Kit-Zeilen: PortalSidebarLink oder PortalSidebarButton nehmen (#5603, .claude/rules/frontend-ui-kit.md).",
    },
  },
  create(context) {
    const key = fileKey(context);
    if (
      !key.startsWith("src/") ||
      EXEMPT_FILE_RE.test(key) ||
      key.startsWith(UI_KIT_DIR)
    ) {
      return {};
    }
    const sourceCode = context.sourceCode ?? context.getSourceCode();
    const check = (node, text) => {
      if (typeof text !== "string") return;
      if (hasPortalNavShellSignature(text)) {
        context.report({ node, messageId: "shell" });
      }
    };
    return {
      Literal: (node) => check(node, node.value),
      TemplateLiteral(node) {
        const chunks = [];
        collectStaticStrings(node, chunks);
        const combinations = classCombinations(
          node,
          new WeakSet(),
          sourceCode,
        ) ?? [chunks.join(" ")];
        const rendered = combinations?.find(hasPortalNavShellSignature);
        if (rendered !== undefined) check(node, rendered);
      },
      JSXAttribute(node) {
        if (
          node.name.type !== "JSXIdentifier" ||
          node.name.name !== "className"
        )
          return;

        const chunks = [];
        collectStaticStrings(node.value, chunks);
        if (chunks.some(hasPortalNavShellSignature)) return;
        const combinations = classCombinations(
          node.value,
          new WeakSet(),
          sourceCode,
        ) ?? [chunks.join(" ")];
        const rendered = combinations.find(hasPortalNavShellSignature);
        if (rendered !== undefined) check(node, rendered);
      },
      ImportSpecifier(node) {
        const name = node.imported?.name;
        if (PORTAL_NAV_KIT_ROW_EXPORTS.has(name)) {
          context.report({ node, messageId: "row", data: { name } });
        }
      },
    };
  },
};

let kitExportNames;

const noShadowKitName = {
  meta: {
    docs: {
      description:
        "Disallow local functions and constants named like a UI-kit export.",
    },
    messages: {
      shadow:
        "`{{name}}` ist ein Export des UI-Kits. Das Kit-Bauteil nutzen oder die lokale Funktion/Konstante nach ihrer Aufgabe benennen (#5583, .claude/rules/frontend-ui-kit.md).",
    },
  },
  create(context) {
    const key = fileKey(context);
    if (
      EXEMPT_FILE_RE.test(key) ||
      key.startsWith(UI_KIT_DIR) ||
      key.startsWith("src/test/")
    ) {
      return {};
    }
    kitExportNames ??= readKitExportNames();
    const check = (identifier) => {
      if (
        identifier?.type === "Identifier" &&
        kitExportNames.has(identifier.name)
      ) {
        context.report({
          node: identifier,
          messageId: "shadow",
          data: { name: identifier.name },
        });
      }
    };
    return {
      FunctionDeclaration: (node) => check(node.id),
      VariableDeclarator: (node) => check(node.id),
    };
  },
};

// Eingabe-Bausteine (#5596): drei Muster, die es im Kit genau einmal gibt.

/** Direkte JSX-Kinder, auch hinter `&&`, `?:`, Fragment und `{…}`. */
function flatJsxChildren(element) {
  const result = [];
  const visit = (node) => {
    if (!node) return;
    if (node.type === "JSXElement") result.push(node);
    else if (node.type === "JSXFragment") node.children.forEach(visit);
    else if (node.type === "JSXExpressionContainer") visit(node.expression);
    else if (node.type === "LogicalExpression") visit(node.right);
    else if (node.type === "ConditionalExpression") {
      visit(node.consequent);
      visit(node.alternate);
    }
  };
  element.children?.forEach(visit);
  return result;
}

function isActionElement(element) {
  const name = jsxName(element.openingElement.name);
  return name === "button" || name === "Button";
}

function actionHoldsIcon(element, iconNames) {
  return (
    isActionElement(element) &&
    flatJsxChildren(element).some((child) =>
      iconNames.has(jsxName(child.openingElement.name) ?? ""),
    )
  );
}

/** Das Kreuz als Symbol oder als Textzeichen „×“. */
function actionRemoves(element) {
  return (
    actionHoldsIcon(element, REMOVE_ICONS) ||
    (isActionElement(element) &&
      element.children.some(
        (child) => child.type === "JSXText" && child.value.trim() === "×",
      ))
  );
}

const REMOVE_ICONS = new Set(["X", "XIcon", "CloseIcon"]);

function kitInputRule({ description, message, kitFile, visitors }) {
  return {
    meta: {
      type: "problem",
      docs: { description },
      messages: { finding: message },
      schema: [],
    },
    create(context) {
      const key = fileKey(context);
      if (
        EXEMPT_FILE_RE.test(key) ||
        key.endsWith(kitFile) ||
        key.startsWith("src/test/")
      ) {
        return {};
      }
      const constants = new Map();
      return {
        VariableDeclarator(node) {
          if (node.id?.type === "Identifier" && node.init) {
            constants.set(node.id.name, node.init);
          }
        },
        ...visitors({
          found: (node) => context.report({ node, messageId: "finding" }),
          textOf: (value) => classText(value, constants),
        }),
      };
    },
  };
}

const noHandRolledRemovableChip = kitInputRule({
  description:
    "A chip with its own remove button is RemovableChip; do not rebuild it.",
  message:
    "Kein eigener Chip mit Entfernen-Kreuz: RemovableChip aus ~/components/ui/removable-chip nehmen (#5596, .claude/rules/frontend-ui-kit.md).",
  kitFile: "src/components/ui/removable-chip.tsx",
  visitors: ({ found, textOf }) => ({
    JSXElement(node) {
      const opening = node.openingElement;
      if (!new Set(["span", "li", "div"]).has(jsxName(opening.name) ?? "")) {
        return;
      }
      const className = jsxAttribute(opening, "className");
      if (!className?.value) return;
      const text = textOf(className.value);
      const isChip =
        /(?:^|\s)inline-flex(?:\s|$)/.test(text) &&
        /(?:^|\s)rounded(?:-[a-z0-9]+)?(?:\s|$)/.test(text) &&
        /(?:^|\s)(?:border|bg-[a-z])/.test(text);
      if (!isChip) return;
      if (flatJsxChildren(node).some(actionRemoves)) found(opening);
    },
  }),
});

const noHandRolledNumberStepper = kitInputRule({
  description:
    "Minus and plus around a number field is NumberStepper; do not rebuild it.",
  message:
    "Kein eigener Zahl-Stepper: NumberStepper aus ~/components/ui/number-stepper nehmen (#5596, .claude/rules/frontend-ui-kit.md).",
  kitFile: "src/components/ui/number-stepper.tsx",
  visitors: ({ found }) => ({
    JSXElement(node) {
      const children = flatJsxChildren(node);
      const hasStep = (icon) =>
        children.some((child) => actionHoldsIcon(child, new Set([icon])));
      if (hasStep("Minus") && hasStep("Plus")) found(node.openingElement);
    },
  }),
});

const WEEKDAY_LIST_RE = /week_?(?:days?|dates)|dayOptions$/i;

/** Das Element, das der `.map`-Callback direkt zurückgibt. */
function returnedJsxElement(body) {
  let node = body;
  if (node?.type === "BlockStatement") {
    node = node.body.find(
      (statement) => statement.type === "ReturnStatement",
    )?.argument;
  }
  while (node?.type === "ParenthesizedExpression") node = node.expression;
  return node?.type === "JSXElement" ? node : null;
}

const noHandRolledWeekdayToggle = kitInputRule({
  description:
    "A row of weekday toggles is WeekdayToggleGroup; do not rebuild it from checkboxes or pressed buttons.",
  message:
    "Keine eigene Wochentagsauswahl: WeekdayToggleGroup aus ~/components/ui/weekday-toggle-group nehmen, für einen Tagesstreifen PlanningDayChip stacked (#5596, .claude/rules/frontend-ui-kit.md).",
  kitFile: "src/components/ui/weekday-toggle-group.tsx",
  visitors: ({ found }) => ({
    CallExpression(node) {
      const callee = node.callee;
      if (
        callee?.type !== "MemberExpression" ||
        callee.property?.name !== "map" ||
        callee.object?.type !== "Identifier" ||
        !WEEKDAY_LIST_RE.test(callee.object.name)
      ) {
        return;
      }
      const element = returnedJsxElement(node.arguments[0]?.body);
      if (!element) return;
      const opening = element.openingElement;
      const name = jsxName(opening.name);
      const isCheckboxLabel =
        name === "label" &&
        flatJsxChildren(element).some(
          (child) => jsxName(child.openingElement.name) === "Checkbox",
        );
      const isPressedButton =
        (name === "button" || name === "Button") &&
        jsxAttribute(opening, "aria-pressed") !== undefined;
      if (isCheckboxLabel || isPressedButton) found(node);
    },
  }),
});

// ui-kit/no-hand-rolled-dropzone und ui-kit/no-hand-rolled-copy-field (#5599).
// Eine Ablagefläche besteht aus Zieh-Zustand, Dateiauswahl und Tastaturweg; die
// sechs Eigenbauten unterschieden sich genau darin. Ein Kopierfeld hat zwei
// Erkennungszeichen: der gerahmte <code>-Wert und der direkte Griff zur
// Zwischenablage, an dem Rückmeldung und Fehlerweg hängen.
const DROPZONE_FILE = "src/components/ui/dropzone.tsx";
const CLIPBOARD_ALLOWED_RE =
  /^src\/(?:components\/ui\/copy-field\.tsx|lib\/use-clipboard-copy\.ts)$/;

const noHandRolledDropzone = {
  meta: {
    type: "problem",
    docs: {
      description: "Disallow hand-built drop areas; use the kit Dropzone.",
    },
    messages: {
      handRolled:
        "Eine Ablagefläche kommt aus dem Kit: Dropzone aus ~/components/ui/dropzone (#5599).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key) || key === DROPZONE_FILE) return {};
    return {
      JSXAttribute(node) {
        if (node.name?.name === "onDrop") {
          context.report({ node, messageId: "handRolled" });
        }
      },
    };
  },
};

const noHandRolledCopyField = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-built copy fields and direct clipboard writes; use CopyField or useClipboardCopy.",
    },
    messages: {
      framedCode:
        "Ein gerahmter Wert zum Kopieren kommt aus dem Kit: CopyField aus ~/components/ui/copy-field (#5599).",
      rawClipboard:
        "Kein direkter Griff zur Zwischenablage: CopyField oder useClipboardCopy, dort hängen Rückmeldung und Fehlerweg (#5599).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key) || CLIPBOARD_ALLOWED_RE.test(key)) return {};
    return {
      JSXOpeningElement(node) {
        if (node.name?.name !== "code") return;
        const className = node.attributes.find(
          (attribute) => attribute.name?.name === "className",
        )?.value?.value;
        if (
          typeof className === "string" &&
          /(?:^|\s)border(?:\s|$)/.test(className) &&
          /(?:^|\s)rounded(?:-\w+)?(?:\s|$)/.test(className)
        ) {
          context.report({ node, messageId: "framedCode" });
        }
      },
      MemberExpression(node) {
        const owner = node.object?.property ?? node.object;
        if (
          owner?.name === "navigator" &&
          node.property?.name === "clipboard"
        ) {
          context.report({ node, messageId: "rawClipboard" });
        }
      },
    };
  },
};

export default {
  meta: { name: "ui-kit" },
  rules: {
    "green-fill-white-text": greenFillWhiteText,
    "no-button-override": noButtonOverride,
    "no-dead-surface-utility": noDeadSurfaceUtility,
    "no-dynamic-color-class": noDynamicColorClass,
    "no-generic-brand-colors": noGenericBrandColors,
    "no-grey-button-fill": noGreyButtonFill,
    "no-hand-rolled-button": noHandRolledButton,
    "no-hand-rolled-copy-field": noHandRolledCopyField,
    "no-hand-rolled-dot": noHandRolledDot,
    "no-hand-rolled-dropzone": noHandRolledDropzone,
    "no-hand-rolled-icon-tile": noHandRolledIconTile,
    "no-hand-rolled-number-stepper": noHandRolledNumberStepper,
    "no-hand-rolled-overlay": noHandRolledOverlay,
    "no-hand-rolled-page-card": noHandRolledPageCard,
    "no-hand-rolled-pill": noHandRolledPill,
    "no-hand-rolled-portal-nav": noHandRolledPortalNav,
    "no-hand-rolled-removable-chip": noHandRolledRemovableChip,
    "no-hand-rolled-surface": noHandRolledSurface,
    "no-hand-rolled-weekday-toggle": noHandRolledWeekdayToggle,
    "no-hex-alpha-tint": noHexAlphaTint,
    "no-control-height-override": noControlHeightOverride,
    "no-important-override": noImportantOverride,
    "no-pill-dot": noPillDot,
    "no-raw-input": noRawInput,
    "no-kit-domain-import": noKitDomainImport,
    "no-raw-z-index": noRawZIndex,
    "no-select-override": noSelectOverride,
    "no-rounded-3xl": noRounded3xl,
    "no-section-card-bleed": noSectionCardBleed,
    "no-status-dot-outside-plans": noStatusDotOutsidePlans,
    "no-shadcn-tokens": noShadcnTokens,
    "no-shadow-kit-name": noShadowKitName,
    "no-tenant-kicker": noTenantKicker,
    "no-tabs-as-value-switcher": noTabsAsValueSwitcher,
    "no-tiny-text": noTinyText,
    "no-white-on-warm-fill": noWhiteOnWarmFill,
    "require-checkbox-label": requireCheckboxLabel,
  },
};
