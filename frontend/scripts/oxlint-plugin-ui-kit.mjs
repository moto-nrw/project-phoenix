import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, normalize } from "node:path/posix";
import { fileURLToPath } from "node:url";

import {
  buttonRatchetAllowlist,
  buttonRatchetBaseline,
} from "./ui-kit-button-baseline.mjs";
import { importantOverrideBaseline } from "./ui-kit-important-override-baseline.mjs";
import { surfaceBaseline } from "./ui-kit-surface-baseline.mjs";

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
//                                     in the kit except SURFACE_KIT_CONTROLS;
//                                     outside the kit also rounded-md/lg,
//                                     bg-white/NN and frames without fill
//                                     (#5594, shrink-only baseline); use
//                                     moto-content-surface (cards),
//                                     AnchoredPopover or OverflowMenu (floating panels),
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
//                                     spelled out outside the kit (all of
//                                     app/ and components/ since #5594,
//                                     shrink-only baseline); use the kit
//                                     cards (unlike
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
// ui-kit/no-uppercase-label (#5590, hard-zero): no `uppercase
// tracking-*` class chain outside the kit; group headings use SectionLabel.
//
// ui-kit/no-hand-rolled-empty-state (#5597, hard-zero): a dashed, centered box
// outside ui/ is a hand-built placeholder; use EmptyState variant="dashed".
//
// Two more keep uploads and copyable values on the kit (#5599), hard-zero:
//
//   ui-kit/no-hand-rolled-dropzone  — no onDrop outside ui/dropzone.tsx; use
//                                     Dropzone
//   ui-kit/no-hand-rolled-copy-field — no framed <code> value and no
//                                     navigator.clipboard outside the kit and
//                                     useClipboardCopy; use CopyField
//
// One more keeps waiting indicators on the kit (#5588), hard-zero:
//
//   ui-kit/no-own-spinner           — no `animate-spin` and no spinner icon
//                                     from an icon package outside
//                                     src/components/ui/; use Loading,
//                                     SpinnerIcon or Button isLoading
//
// One more keeps form fields on the kit (#5217):
//
//   ui-kit/no-kit-domain-import     — files in components/ui import no other
//                                     components and fetch no data (#5586)
//   ui-kit/no-raw-table             — no <table> outside components/ui; use
//                                     DataTable (#5600)
//   ui-kit/no-em-dash-empty-value   — an empty value is "–", never "—" (#5600)
//   ui-kit/no-radix-outside-kit     — @radix-ui is an implementation detail of
//                                     components/ui; everyone else imports the
//                                     kit component. Hard-zero (#5604).
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
// Four keep the kit variants of #5606 in use, hard-zero:
//
//   ui-kit/no-tabs-override         — no look classes on TabsList/TabsTrigger;
//                                     pick a variant, pass layout only
//   ui-kit/no-section-card-title-size — no text size in SectionCard's
//                                     titleClassName; use titleSize
//   ui-kit/no-empty-state-surface   — no card classes on EmptyState; the card
//                                     comes from SectionCard or the page's
//                                     `empty`
//   ui-kit/no-hand-rolled-avatar    — no tinted circle around initials; use
//                                     Avatar
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

/**
 * Resolves the initializer of the exact lexical binding, including a binding
 * declared later in the module. Falling back to the traversal-built map would
 * make class checks depend on declaration order and can pick a shadowed name.
 */
function constantInitializer(identifier, constants, sourceCode) {
  if (!sourceCode) return constants.get(identifier.name);
  const variable = findVariable(sourceCode, identifier);
  if (!variable) return undefined;
  for (const definition of variable.defs) {
    if (
      definition.type === "Variable" &&
      definition.node.type === "VariableDeclarator" &&
      definition.node.init
    ) {
      return definition.node.init;
    }
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
  // Conditional alternatives need independent copies of the traversal state.
  seen = new Set(),
  sourceCode,
  seenVariables = new Set(),
  { skipReportedSurfaceConstants = false } = {},
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
      const value = staticStringValue(definition.node.init);
      if (
        skipReportedSurfaceConstants &&
        typeof value === "string" &&
        isSurfaceCombo(value)
      ) {
        continue;
      }
      const resolved = classCombinations(
        definition.node.init,
        seen,
        sourceCode,
        seenVariables,
        { skipReportedSurfaceConstants },
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
      new Set(seen),
      sourceCode,
      new Set(seenVariables),
      { skipReportedSurfaceConstants },
    );
    const alternate = classCombinations(
      node.alternate,
      new Set(seen),
      sourceCode,
      new Set(seenVariables),
      { skipReportedSurfaceConstants },
    );
    if (!consequent || !alternate) return null;
    const merged = [...consequent, ...alternate];
    return merged.length > MAX_CLASS_COMBINATIONS ? null : merged;
  }
  if (node.type === "LogicalExpression") {
    const right = classCombinations(
      node.right,
      new Set(seen),
      sourceCode,
      new Set(seenVariables),
      { skipReportedSurfaceConstants },
    );
    if (!right) return null;
    if (node.operator === "&&") return ["", ...right];
    const left = classCombinations(
      node.left,
      new Set(seen),
      sourceCode,
      new Set(seenVariables),
      { skipReportedSurfaceConstants },
    );
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
        { skipReportedSurfaceConstants },
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

// #5594 schärft die Regel außerhalb des Kits: eine Fläche ist auch
// `rounded-md`/`rounded-lg` mit Rahmen und weißer Füllung, eine halbdurchsichtige
// weiße Füllung (`bg-white/80`) und ein Rahmen mit Polster ganz ohne Füllung.
// Geprüft wird das Element mit seinem className (auch über Konstanten); Knöpfe,
// Links und Felder sind Bedienelemente, keine Fläche (eigene Regeln), ein
// gestrichelter Rahmen ist EmptyState/InsetPanel, `moto-*-surface` ist die
// Kit-Fläche selbst. Ein Rahmen ohne Füllung zählt nur mit Polster: ohne
// Polster ist er die Hülle einer Liste oder Tabelle (FramedList). Das Kit
// bleibt bei der alten Kette, es definiert die Bedienelemente. Die alte
// Kette ist weiter hart null; nur die Schärfung trägt eine Baseline.
const SURFACE_WIDE_ROUNDED_RE = /^rounded-(?:md|lg|xl|2xl)$/;
const SURFACE_FRAME_RE = /^border(?:-[1-9]\d*)?$/;
const SURFACE_WHITE_FILL_RE = /^bg-white(?:\/[1-9]\d*)?$/;
const SURFACE_RESPONSIVE_VARIANT_RE =
  /^(?:(?:max-)?(?:sm|md|lg|xl|2xl)|compact|min-\[[^\]]+\]|max-\[[^\]]+\])$/;
const SURFACE_BREAKPOINT_VALUES = new Map([
  ["sm", 640],
  ["md", 768],
  ["lg", 1024],
  ["xl", 1280],
  ["2xl", 1536],
]);
const SURFACE_ARBITRARY_BREAKPOINT_RE =
  /^(min|max)-\[(\d+(?:\.\d+)?)(?:px|rem|em)\]$/;
const SURFACE_INTERACTION_VARIANT_RE = /^(?:hover|focus(?:-visible|-within)?)$/;
const SURFACE_CONTROL_ELEMENTS = new Set([
  "a",
  "button",
  "input",
  "label",
  "select",
  "textarea",
  "Link",
  "NavLink",
  "NavigationLink",
  "Button",
  "ButtonLink",
  "ListRowLink",
  "ChoiceTile",
]);

function surfaceResponsiveRange(variant) {
  const min = SURFACE_BREAKPOINT_VALUES.get(variant);
  if (min !== undefined) return { min };

  const max = variant.startsWith("max-")
    ? SURFACE_BREAKPOINT_VALUES.get(variant.slice(4))
    : undefined;
  if (max !== undefined) return { max };

  const arbitrary = SURFACE_ARBITRARY_BREAKPOINT_RE.exec(variant);
  if (!arbitrary) return null;
  const [, direction, value, unit] = arbitrary;
  const pixels = Number(value) * (unit === "px" ? 1 : 16);
  return direction === "min" ? { min: pixels } : { max: pixels };
}

function splitTailwindVariants(token) {
  const parts = [];
  let start = 0;
  let bracketDepth = 0;
  for (let index = 0; index < token.length; index++) {
    if (token[index] === "[") bracketDepth++;
    if (token[index] === "]") bracketDepth--;
    if (token[index] !== ":" || bracketDepth !== 0) continue;
    parts.push(token.slice(start, index));
    start = index + 1;
  }
  parts.push(token.slice(start));
  return parts;
}

function surfaceResponsiveToken(token) {
  const parts = splitTailwindVariants(token);
  if (parts.length === 1) return { utility: token };

  const utility = parts.pop();
  const variants = parts;
  const ranges = [];
  let compact = false;
  let interaction;
  for (const variant of variants) {
    const range = surfaceResponsiveRange(variant);
    if (range) {
      ranges.push(range);
      continue;
    }
    if (variant === "compact") {
      compact = true;
      continue;
    }
    if (SURFACE_INTERACTION_VARIANT_RE.test(variant)) {
      interaction ??= variant;
      continue;
    }
    return null;
  }
  if (ranges.length === 0 && !compact) return null;
  return {
    utility:
      interaction && utility.startsWith("bg-")
        ? `${interaction}:${utility}`
        : utility,
    ranges,
    compact,
  };
}

function isWideSurfaceTokens(tokens) {
  const baseTokens = [];
  const responsiveTokens = [];
  const isolatedTokens = new Map();
  for (const token of tokens) {
    const parsed = surfaceResponsiveToken(token);
    if (parsed && parsed.ranges === undefined) {
      baseTokens.push(parsed.utility);
      continue;
    }
    if (parsed) {
      responsiveTokens.push(parsed);
      continue;
    }

    const [variant, utility, ...rest] = splitTailwindVariants(token);
    if (
      rest.length === 0 &&
      utility &&
      SURFACE_RESPONSIVE_VARIANT_RE.test(variant)
    ) {
      const variantTokens = isolatedTokens.get(variant) ?? [];
      variantTokens.push(utility);
      isolatedTokens.set(variant, variantTokens);
    }
  }
  const states = [];
  const bounds = new Set([0]);
  for (const { ranges, compact } of responsiveTokens) {
    for (const range of ranges) bounds.add(range.min ?? range.max);
    // `compact` starts at `md` and can also match short wide screens.
    if (compact) bounds.add(SURFACE_BREAKPOINT_VALUES.get("md"));
  }
  for (const width of [...bounds].sort((left, right) => left - right)) {
    const state = [...baseTokens];
    for (const { utility, ranges, compact } of responsiveTokens) {
      if (
        (!compact || width >= SURFACE_BREAKPOINT_VALUES.get("md")) &&
        ranges.every(
          (range) =>
            (range.min === undefined || width >= range.min) &&
            (range.max === undefined || width < range.max),
        )
      ) {
        state.push(utility);
      }
    }
    states.push(state);
  }
  for (const variantTokens of isolatedTokens.values()) {
    states.push([...baseTokens, ...variantTokens]);
  }
  return states.some(isWideSurfaceTokenGroup);
}

function isWideSurfaceTokenGroup(tokens) {
  if (!tokens.some((token) => SURFACE_WIDE_ROUNDED_RE.test(token))) {
    return false;
  }
  if (!tokens.some((token) => SURFACE_FRAME_RE.test(token))) return false;
  if (tokens.includes("border-transparent")) return false;
  if (tokens.some((token) => /^border-l-\[/.test(token))) return false;
  if (
    tokens.some(
      (token) =>
        token === "border-dashed" || /^moto-[\w-]*surface$/.test(token),
    )
  ) {
    return false;
  }
  if (tokens.some((token) => SURFACE_WHITE_FILL_RE.test(token))) return true;
  const anyFill = tokens.some(
    (token) =>
      /(?:^|:)bg-/.test(token) &&
      !/(?:^|:)(?:hover|focus(?:-visible|-within)?):bg-/.test(token) &&
      !/(?:^|:)bg-transparent$/.test(token),
  );
  const padded = tokens.some((token) => /^p[xytblrse]?-\d/.test(token));
  return !anyFill && padded;
}

/**
 * Shrink-only Baseline je Datei für Regeln mit eigenem Besucher (#5594): die
 * Funde aus `count` werden gesammelt und erst gemeldet, wenn eine Datei mehr
 * hat, als `baselines[ruleName][file]` erlaubt. Ein Fund hinter
 * `oxlint-disable-next-line <rule>` zählt nicht mit, genau wie im Generator.
 * Mit `countEnv=count` meldet die Regel jeden Fund; daraus schreibt
 * `pnpm run ui-kit:surface-baseline` die Zahlen.
 */
function withFileBaseline({ ruleName, baselines, countEnv }, create) {
  return (context) => {
    const key = fileKey(context);
    const findings = [];
    const visitors = create(context, (descriptor) => findings.push(descriptor));
    if (!visitors || Object.keys(visitors).length === 0) return {};
    const exit = visitors["Program:exit"];
    return {
      ...visitors,
      "Program:exit"(node) {
        exit?.(node);
        const disabledLines = new Set();
        for (const comment of context.sourceCode?.getAllComments?.() ?? []) {
          if (
            comment.value.includes("oxlint-disable-next-line") &&
            comment.value.includes(`ui-kit/${ruleName}`)
          ) {
            disabledLines.add(comment.loc.end.line + 1);
          }
        }
        const counted = findings.filter(
          (descriptor) => !disabledLines.has(descriptor.node.loc?.start.line),
        );
        const baseline = baselines[ruleName]?.[key] ?? 0;
        const countMode = process.env[countEnv] === "count";
        if (!countMode && counted.length <= baseline) return;
        for (const descriptor of counted) {
          context.report(
            countMode || baseline === 0
              ? descriptor
              : {
                  ...descriptor,
                  messageId: "overBaseline",
                  data: {
                    found: String(counted.length),
                    baseline: String(baseline),
                  },
                },
          );
        }
      },
    };
  };
}

const SURFACE_OVER_BASELINE =
  "Diese Datei hat {{found}} handgebaute Flächen, die Baseline erlaubt {{baseline}}. Neue Flächen aus dem Kit bauen: SectionCard, TileCard, InsetPanel oder moto-content-surface (#5594, Abbau: pnpm run ui-kit:surface-baseline).";

const noHandRolledSurface = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-built card surfaces (`rounded-xl/2xl` + `border` + `bg-white`), in the UI kit too (#5586); outside the kit also `rounded-md/lg`, translucent white fills and frames without fill (#5594). Use moto-content-surface, moto-popover-surface, ChoiceTile or a kit surface component (SectionCard).",
    },
    messages: {
      handRolledSurface:
        "Hand-rolled card surface ('{{match}}'). Use moto-content-surface (cards), AnchoredPopover or OverflowMenu (floating panels), ChoiceTile (selectable rows and tiles) or a kit surface component (SectionCard) instead. A deliberate non-card match may stay behind `// oxlint-disable-next-line ui-kit/no-hand-rolled-surface` with the reason recorded in the PR description (issue #2933).",
      overBaseline: SURFACE_OVER_BASELINE,
    },
    schema: [],
  },
  create: withFileBaseline(
    {
      ruleName: "no-hand-rolled-surface",
      baselines: surfaceBaseline,
      countEnv: "UI_KIT_SURFACE_RATCHET",
    },
    (context, report) => {
      const key = fileKey(context);
      const countMode = process.env.UI_KIT_SURFACE_RATCHET === "count";
      if (!key.startsWith("src/")) return {};
      if (EXEMPT_FILE_RE.test(key)) return {};
      if (SURFACE_KIT_CONTROLS.has(key)) return {};
      // Die Eltern-Ausnahmen aus #2619 gelten nur für die Schärfung; die
      // alte Kette bleibt auch dort hart null.
      const widened =
        !key.startsWith(UI_KIT_DIR) && !INSET_PARENT_EXCEPTIONS.includes(key);

      // Die alte Kette ist hart null und meldet sofort, an der Baseline vorbei.
      const check = (node, text) => {
        if (!isSurfaceCombo(text)) return;
        // Der Generator zählt ausschließlich die shrink-only Schärfung.
        if (countMode) return;
        context.report({
          node,
          messageId: "handRolledSurface",
          data: { match: "rounded-xl/2xl … border … bg-white" },
        });
      };
      const surfaceComboKey = (text) => text.trim().replace(/\s+/g, " ");
      // Literal-/Template- und konstante cn()-Treffer melden sich schon an
      // ihrer Definition. Die Verwendungen einer Konstanten dürfen diese
      // alte Kette nicht nochmals melden, wohl aber einen anderen, erweiterten
      // Zweig derselben className-Expression.
      const separatelyReportedSurfaceCombos = (
        value,
        seenVariables = new Set(),
        seenNodes = new WeakSet(),
      ) => {
        const reported = new Set();
        const collect = (node) => {
          if (!node || typeof node !== "object" || seenNodes.has(node)) return;
          seenNodes.add(node);
          if (
            node.type === "JSXExpressionContainer" ||
            node.type === "ChainExpression" ||
            node.type === "TSAsExpression" ||
            node.type === "TSTypeAssertion" ||
            node.type === "TypeCastExpression"
          ) {
            collect(node.expression);
            return;
          }
          if (node.type === "Literal" && typeof node.value === "string") {
            if (isSurfaceCombo(node.value)) {
              reported.add(surfaceComboKey(node.value));
            }
            return;
          }
          if (node.type === "TemplateLiteral") {
            for (const quasi of node.quasis) {
              if (isSurfaceCombo(quasi.value.raw)) {
                reported.add(surfaceComboKey(quasi.value.raw));
              }
            }
            return;
          }
          if (node.type === "Identifier") {
            const variable = findVariable(context.sourceCode, node);
            if (!variable || seenVariables.has(variable)) return;
            seenVariables.add(variable);
            for (const definition of variable.defs) {
              if (
                definition.type !== "Variable" ||
                definition.node.type !== "VariableDeclarator" ||
                definition.parent?.kind !== "const" ||
                !definition.node.init
              )
                continue;
              const initializer = definition.node.init;
              if (initializer.type === "CallExpression") {
                const combinations = classCombinations(
                  initializer,
                  new Set(),
                  context.sourceCode,
                );
                for (const combo of combinations ?? []) {
                  if (isSurfaceCombo(combo))
                    reported.add(surfaceComboKey(combo));
                }
              }
              collect(initializer);
            }
            return;
          }
          if (node.type === "ConditionalExpression") {
            collect(node.consequent);
            collect(node.alternate);
            return;
          }
          if (node.type === "LogicalExpression") {
            collect(node.left);
            collect(node.right);
          }
        };
        collect(value);
        return reported;
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
        VariableDeclarator(node) {
          if (
            node.parent?.kind !== "const" ||
            node.init?.type !== "CallExpression"
          )
            return;
          const chunks = [];
          collectStaticStrings(node.init, chunks);
          if (chunks.some(isSurfaceCombo)) return;
          const combos = classCombinations(
            node.init,
            new Set(),
            context.sourceCode,
          );
          const rendered = combos?.find((combo) => isSurfaceCombo(combo));
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
          if (chunks.some(isSurfaceCombo)) return;
          const reported = separatelyReportedSurfaceCombos(node.value);
          const combinations = classCombinations(
            node.value,
            new Set(),
            context.sourceCode,
          ) ?? [chunks.join(" ")];
          for (const combo of new Set(combinations)) {
            if (
              isSurfaceCombo(combo) &&
              !reported.has(surfaceComboKey(combo))
            ) {
              check(node.parent ?? node, combo);
            }
          }
        },
        JSXOpeningElement(node) {
          if (!widened) return;
          if (SURFACE_CONTROL_ELEMENTS.has(jsxName(node.name) ?? "")) return;
          const className = jsxAttribute(node, "className");
          if (!className?.value) return;
          const chunks = [];
          collectStaticStrings(className.value, chunks);
          const combinations = classCombinations(
            className.value,
            new Set(),
            context.sourceCode,
          ) ?? [chunks.join(" ")];
          if (
            combinations.some(
              (combo) =>
                !isSurfaceCombo(combo) &&
                isWideSurfaceTokens(combo.split(/\s+/)),
            )
          ) {
            report({
              node,
              messageId: "handRolledSurface",
              data: {
                match:
                  "rounded-md…2xl … border … bg-white(/NN) oder Rahmen mit Polster",
              },
            });
          }
        },
      };
    },
  ),
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
            : "AnchoredPopover",
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
        const combos = classCombinations(node.value, new Set(), sourceCode) ?? [
          chunks.join(" "),
        ];
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
        const combos = classCombinations(node.value, new Set(), sourceCode) ?? [
          chunks.join(" "),
        ];
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
  "src/components/guardians/guardian-list.tsx",
  "src/components/parent/calendar/parent-calendar-page.tsx",
  "src/components/parent/parent-meal-plan-page.tsx",
  "src/components/staff/absence-request-row.tsx",
  "src/components/staff/dienstplan-resource-grid.tsx",
  "src/components/staff/staff-session-table.tsx",
  "src/components/students/care-schedule-manager.tsx",
  "src/components/students/planned-status-days-modal.tsx",
  "src/components/students/school-checkin-fab.tsx",
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
  "src/components/ui/page-header/OverflowMenu.tsx",
  "src/components/ui/planning/plan-block.tsx",
  "src/components/ui/planning/plan-legend.tsx",
  "src/components/ui/presence-badge.tsx",
]);

const TINY_TEXT_BASELINE = parseLocationBaseline(`
src/app/[tenant]/(protected)/time-tracking/week-chart.tsx|text-[10px]@198
src/app/help/nfc/erste-schritte/page.tsx|text-[11px]@226 text-[11px]@275
src/app/operator/provisioning/soft-delete-shared.tsx|text-[11px]@238
src/components/auth/role-permission-management-modal.tsx|text-[10px]@330 text-[11px]@375
src/components/dashboard/header/reminders-bell.tsx|text-[11px]@46
src/components/enrollment/admin-enrollments-list.tsx|text-[11px]@959
src/components/enrollment/enrollment-form.tsx|text-[11px]@2189
src/components/files/files-page.tsx|text-[11px]@181
src/components/guardians/guardian-list.tsx|text-[10px]@468
src/components/parent/calendar/parent-calendar-page.tsx|text-[11px]@717 text-[11px]@783
src/components/parent/parent-meal-plan-page.tsx|text-[11px]@461 text-[11px]@546
src/components/staff/absence-request-row.tsx|text-[11px]@107
src/components/staff/dienstplan-resource-grid.tsx|text-[11px]@489 text-[11px]@585 text-[11px]@594 text-[11px]@601
src/components/staff/staff-session-table.tsx|text-[11px]@1194
src/components/students/planned-status-days-modal.tsx|text-[10px]@935 text-[11px]@935
src/components/students/school-checkin-fab.tsx|text-[10px]@200
src/components/time-tracking/vacation-request-modal.tsx|text-[11px]@372
src/components/timetable/bulk-substitution-modal.tsx|text-[11px]@530
src/components/timetable/event-form/multi-select-field.tsx|text-[11px]@147 text-[11px]@312
src/components/timetable/event-form/step-termin.tsx|text-[10px]@170
src/components/timetable/gap-jump-list.tsx|text-[11px]@132 text-[11px]@147
src/components/timetable/instance-block.tsx|text-[10px]@298 text-[10px]@332 text-[10px]@339 text-[10px]@349
src/components/timetable/instance-detail-modal.tsx|text-[11px]@1388 text-[11px]@1541 text-[11px]@1669
src/components/timetable/month-planner-grid.tsx|text-[11px]@142 text-[11px]@151 text-[11px]@172 text-[10px]@238
src/components/timetable/period-switcher-dropdown.tsx|text-[11px]@179 text-[11px]@255 text-[10px]@321 text-[11px]@398 text-[11px]@405
src/components/timetable/staff-pool-slide-over.tsx|text-[11px]@482
src/components/timetable/substitution-person-card.tsx|text-[11px]@59 text-[11px]@338 text-[11px]@345 text-[11px]@352 text-[11px]@360
src/components/timetable/substitution-slide-over.tsx|text-[11px]@744
src/components/timetable/template-card.tsx|text-[10px]@198
src/components/timetable/vertretung-day-list.tsx|text-[11px]@348 text-[11px]@388 text-[11px]@395 text-[11px]@416 text-[11px]@422
src/components/timetable/weekly-calendar-grid.tsx|text-[11px]@461 text-[10px]@484 text-[10px]@519 text-[11px]@519
src/components/ui/avatar.tsx|text-[10px]@32
src/components/ui/planning/coverage-indicator.tsx|text-[11px]@61 text-[11px]@163
src/components/ui/location-badge.tsx|text-[10px]@282
src/components/ui/notification-badge.tsx|text-[10px]@18
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
// selben Slot). Kein Baseline-Eintrag: der Bestand ist auf null gebracht. Das
// Schul- und das Operator-Portal gehören seit #5604 dazu; nur das Elternportal
// behält seinen Kicker (ADR 0149).
const OTHER_PORTALS_RE = /(^|\/)(components\/parent|app\/parents)\//;

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
// StatCard (Kennzahl), TenantPageHeaderSkeleton (Ladezustand). Nur das Kit
// darf die Fläche definieren, es ist die Quelle.
// Nur die KARTE ist gemeint: `moto-content-surface` zusammen mit dem
// Kartenradius. Dieselbe Klasse an einer Pille oder einem Auswahlfeld
// (rounded-full, h-9) ist eine Bedienfläche und bleibt erlaubt.
const HANDROLLED_SURFACE_RE =
  /moto-content-surface[^"'`]*rounded-2xl|rounded-2xl[^"'`]*moto-content-surface/;

// #5594: die Regel gilt für jede `.tsx` unter `app/` und `components/` außer dem
// Kit, nicht mehr nur für Seitendateien. Der Bestand steht in der shrink-only
// Baseline (ui-kit-surface-baseline.json); die Eltern-Ausnahmen aus #2619 sind
// dieselben wie bei no-hand-rolled-inset.
function isPageCardScope(key) {
  if (!/\.tsx$/.test(key) || EXEMPT_FILE_RE.test(key)) return false;
  if (key.startsWith(UI_KIT_DIR)) return false;
  if (INSET_PARENT_EXCEPTIONS.includes(key)) return false;
  return key.startsWith("src/app/") || key.startsWith("src/components/");
}

const noHandRolledPageCard = {
  meta: {
    docs: {
      description:
        "Disallow hand-rolled card surfaces outside the UI kit; use the kit cards.",
    },
    messages: {
      surface:
        "Keine eigene Kartenfläche. SectionCard (Fläche, mit oder ohne Titel), TileCard (anklickbare Kachel), StatCard (Kennzahl) oder TenantPageHeaderSkeleton nehmen; innere Karten `moto-content-surface rounded-xl` (siehe .claude/rules/frontend-ui-kit.md, #5594).",
      overBaseline: SURFACE_OVER_BASELINE,
    },
  },
  create: withFileBaseline(
    {
      ruleName: "no-hand-rolled-page-card",
      baselines: surfaceBaseline,
      countEnv: "UI_KIT_SURFACE_RATCHET",
    },
    (context, report) => {
      if (!isPageCardScope(fileKey(context))) return {};
      const isReportedPageCardConstant = (value, seenVariables = new Set()) => {
        if (value?.type === "JSXExpressionContainer") {
          return isReportedPageCardConstant(value.expression, seenVariables);
        }
        if (value?.type !== "Identifier") return false;
        const variable = findVariable(context.sourceCode, value);
        if (!variable || seenVariables.has(variable)) return false;
        seenVariables.add(variable);
        for (const definition of variable.defs) {
          if (
            definition.type !== "Variable" ||
            definition.node.type !== "VariableDeclarator" ||
            definition.parent?.kind !== "const" ||
            !definition.node.init
          )
            continue;
          const initializer = definition.node.init;
          if (
            HANDROLLED_SURFACE_RE.test(staticStringValue(initializer) ?? "")
          ) {
            return true;
          }
          if (initializer.type === "CallExpression") {
            const combinations = classCombinations(
              initializer,
              new Set(),
              context.sourceCode,
            );
            if (
              combinations?.some((combo) => HANDROLLED_SURFACE_RE.test(combo))
            ) {
              return true;
            }
          }
          if (isReportedPageCardConstant(initializer, seenVariables))
            return true;
        }
        return false;
      };

      return {
        Literal(node) {
          if (
            typeof node.value === "string" &&
            HANDROLLED_SURFACE_RE.test(node.value)
          ) {
            report({ node, messageId: "surface" });
          }
        },
        TemplateElement(node) {
          const raw = node.value?.cooked ?? node.value?.raw ?? "";
          if (HANDROLLED_SURFACE_RE.test(raw)) {
            report({ node, messageId: "surface" });
          }
        },
        VariableDeclarator(node) {
          if (
            node.parent?.kind !== "const" ||
            node.init?.type !== "CallExpression"
          )
            return;
          const chunks = [];
          collectStaticStrings(node.init, chunks);
          if (chunks.some((chunk) => HANDROLLED_SURFACE_RE.test(chunk))) {
            return;
          }
          const combos = classCombinations(
            node.init,
            new Set(),
            context.sourceCode,
          );
          if (combos?.some((combo) => HANDROLLED_SURFACE_RE.test(combo))) {
            report({ node, messageId: "surface" });
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
          if (isReportedPageCardConstant(node.value)) return;
          const combos = classCombinations(
            node.value,
            new Set(),
            context.sourceCode,
          ) ?? [chunks.join(" ")];
          const rendered = combos.find((combo) =>
            HANDROLLED_SURFACE_RE.test(combo),
          );
          if (rendered !== undefined) {
            report({ node, messageId: "surface" });
          }
        },
      };
    },
  ),
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

// --- ui-kit/no-radix-outside-kit ----------------------------------------------
//
// Radix is what the kit builds on, not a second kit. A portal page that reaches
// for a primitive directly bypasses the kit's styling and behaviour next to the
// kit parts it renders anyway (`TabsPrimitive.Content` beside `Tabs`, #5604).

const noRadixOutsideKit = {
  meta: {
    type: "problem",
    docs: {
      description: "Only files under components/ui import from @radix-ui.",
    },
    messages: {
      radix:
        "'{{source}}' nur im Kit importieren. Außerhalb von components/ui den Kit-Baustein nehmen (Reiter: Tabs, TabsContent aus ~/components/ui/tabs), siehe .claude/rules/frontend-ui-kit.md.",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (key.startsWith(UI_KIT_DIR) || EXEMPT_FILE_RE.test(key)) return {};

    const checkSource = (node, source) => {
      if (typeof source !== "string" || !source.startsWith("@radix-ui/")) {
        return;
      }
      context.report({ node, messageId: "radix", data: { source } });
    };
    const checkDeclaration = (node) => checkSource(node, node.source?.value);

    return {
      ImportDeclaration: checkDeclaration,
      ExportNamedDeclaration: checkDeclaration,
      ExportAllDeclaration: checkDeclaration,
      ImportExpression(node) {
        checkSource(node, node.source?.value);
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

// --- ui-kit/no-raw-table + ui-kit/no-em-dash-empty-value (#5600) -------------

const noRawTable = {
  meta: {
    type: "problem",
    docs: {
      description: "Build tables from DataTable instead of a raw <table>.",
    },
    messages: {
      rawTable:
        "Keine eigene <table>. DataTable aus ~/components/ui/data-table nehmen; expandable, renderSubRows, summaryRow und die Spalte actions decken Aufklappen, Unterzeilen, Summen und die haftende Aktionsspalte ab (#5600, .claude/rules/frontend-ui-kit.md).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (EXEMPT_FILE_RE.test(key) || key.includes("src/components/ui/")) {
      return {};
    }
    return {
      JSXOpeningElement(node) {
        if (jsxName(node.name) === "table") {
          context.report({ node, messageId: "rawTable" });
        }
      },
    };
  },
};

const EM_DASH = String.fromCodePoint(0x2014);
// The dash alone or as the value after a label ("Abholzeit: —").
const EM_DASH_VALUE_RE = new RegExp(`^\\s*(?:\\S.*:\\s*)?${EM_DASH}\\s*$`);

const noEmDashEmptyValue = {
  meta: {
    type: "problem",
    docs: {
      description: "An empty value shows an en dash, not an em dash.",
    },
    messages: {
      emDash:
        "Leerwert ist „–“ (Halbgeviertstrich), nicht „—“ (#5600, .claude/rules/frontend-ui-kit.md).",
    },
    schema: [],
  },
  create(context) {
    if (EXEMPT_FILE_RE.test(fileKey(context))) return {};
    return {
      JSXText(node) {
        if (EM_DASH_VALUE_RE.test(node.value)) {
          context.report({ node, messageId: "emDash" });
        }
      },
      Literal(node) {
        if (
          typeof node.value === "string" &&
          EM_DASH_VALUE_RE.test(node.value)
        ) {
          context.report({ node, messageId: "emDash" });
        }
      },
      TemplateLiteral(node) {
        const text = node.quasis[0]?.value.cooked ?? "";
        if (node.quasis.length === 1 && EM_DASH_VALUE_RE.test(text)) {
          context.report({ node, messageId: "emDash" });
        }
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
 * "…"`) are followed one level, including bindings declared later in the
 * module, so a class chain moved into a variable still counts.
 * `buttonClassName(…)` is the kit itself and contributes nothing here; its
 * className goes through no-button-override.
 */
function classText(
  node,
  constants,
  sourceCode,
  { skipReportedSurfaceConstants = false } = {},
) {
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
      const init = constantInitializer(current, constants, sourceCode);
      const value = staticStringValue(init);
      if (
        skipReportedSurfaceConstants &&
        typeof value === "string" &&
        isSurfaceCombo(value)
      ) {
        return;
      }
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

/**
 * Statische Klassenalternativen eines Ausdrucks. Anders als `classText()`
 * bleiben die Seiten eines Ternärausdrucks getrennt: `a ? "uppercase" :
 * "tracking-wide"` ist keine Versalien-Beschriftung, weil beide Klassen nie
 * gemeinsam gerendert werden. Aufruf- und Array-Argumente gehören dagegen
 * zur selben möglichen Klassenkette und werden miteinander kombiniert.
 */
function classTextAlternatives(node, constants, depth = 0) {
  if (!node || typeof node !== "object") return [""];
  if (
    node.type === "JSXExpressionContainer" ||
    node.type === "ChainExpression" ||
    node.type === "TSAsExpression" ||
    node.type === "TSTypeAssertion" ||
    node.type === "TypeCastExpression"
  ) {
    return classTextAlternatives(node.expression, constants, depth);
  }
  if (node.type === "Literal") {
    return typeof node.value === "string" ? [node.value] : [""];
  }
  if (node.type === "Identifier") {
    const init = depth < 2 ? constants.get(node.name) : undefined;
    return init ? classTextAlternatives(init, constants, depth + 1) : [""];
  }
  if (node.type === "ConditionalExpression") {
    return [
      ...classTextAlternatives(node.consequent, constants, depth),
      ...classTextAlternatives(node.alternate, constants, depth),
    ];
  }
  if (node.type === "LogicalExpression") {
    if (node.operator === "&&") {
      return ["", ...classTextAlternatives(node.right, constants, depth)];
    }
    if (node.operator === "||" || node.operator === "??") {
      return [
        ...classTextAlternatives(node.left, constants, depth),
        ...classTextAlternatives(node.right, constants, depth),
      ];
    }
  }
  if (node.type === "TemplateLiteral") {
    let alternatives = [node.quasis[0]?.value.raw ?? ""];
    for (const [index, expression] of node.expressions.entries()) {
      alternatives = combineClassTextAlternatives(
        alternatives,
        classTextAlternatives(expression, constants, depth),
      ).map(
        (alternative) =>
          `${alternative}${node.quasis[index + 1]?.value.raw ?? ""}`,
      );
    }
    return alternatives;
  }
  if (node.type === "CallExpression") {
    if (
      node.callee?.type === "Identifier" &&
      node.callee.name === "buttonClassName"
    ) {
      return [""];
    }
    return node.arguments.reduce(
      (alternatives, argument) =>
        combineClassTextAlternatives(
          alternatives,
          classTextAlternatives(argument, constants, depth),
        ),
      [""],
    );
  }
  if (node.type === "ArrayExpression") {
    return node.elements.reduce(
      (alternatives, element) =>
        combineClassTextAlternatives(
          alternatives,
          classTextAlternatives(element, constants, depth),
        ),
      [""],
    );
  }
  return [""];
}

function combineClassTextAlternatives(left, right) {
  return [...new Set(left.flatMap((a) => right.map((b) => `${a} ${b}`)))];
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
  exemptDirs = [],
  exemptFiles = [],
  followStaticClassSources = false,
  onlySrc = false,
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
        exemptDirs.some((dir) => key.startsWith(dir)) ||
        exemptFiles.includes(key) ||
        (onlySrc && !key.startsWith("src/")) ||
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
          new Set(),
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

// ui-kit/no-hand-rolled-sticky-bar — eine klebende Fläche mit Aktionen kommt aus
// dem Kit (#5602): unten `StickyActionBar` oder `SelectionBar`, oben `SelectionBar`.
// Die bündige Fußzeile von DatabaseForm bleibt die eine benannte Ausnahme: Sie
// schwebt nicht, sondern schließt den Rahmen des Formulars ab; einen Baustein
// dafür sieht #5602 nicht vor.
const STICKY_BAR_ALLOWED_FILES = new Set([
  "src/components/database/database-form.tsx",
]);

function isHandRolledStickyBar(text) {
  // Breakpoint-Präfixe zählen mit: `lg:bottom-4` ist dieselbe Leiste.
  const tokens = text.split(/\s+/).map(bareUtility);
  if (!tokens.includes("sticky")) return false;
  const surface = tokens.includes("moto-content-surface");
  if (surface && tokens.some((token) => token.startsWith("top-"))) return true;
  return (
    tokens.some((token) => token.startsWith("bottom-")) &&
    (surface || tokens.some((token) => /^(?:bg-|shadow|border)/.test(token)))
  );
}

const noHandRolledStickyBar = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-built sticky action surfaces outside the UI kit; use StickyActionBar or SelectionBar.",
    },
    messages: {
      handRolled:
        "Keine eigene klebende Leiste: StickyActionBar (~/components/ui/sticky-action-bar) für die Aktionen einer Seite, SelectionBar (~/components/ui/selection-bar) für eine Mehrfachauswahl (#5602).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key) || key.startsWith(UI_KIT_DIR)) return {};
    if (STICKY_BAR_ALLOWED_FILES.has(key)) return {};
    const check = (node, text) => {
      if (isHandRolledStickyBar(text)) {
        context.report({ node, messageId: "handRolled" });
      }
    };
    return {
      Literal(node) {
        if (typeof node.value === "string") check(node, node.value);
      },
      TemplateLiteral(node) {
        for (const quasi of node.quasis) check(node, quasi.value.raw);
      },
      // `cn("sticky bottom-4", "bg-white")`: die Teile ergeben erst zusammen die
      // Leiste.
      JSXAttribute(node) {
        if (node.name.type !== "JSXIdentifier") return;
        if (node.name.name !== "className") return;
        const chunks = [];
        collectStaticStrings(node.value, chunks);
        if (chunks.some(isHandRolledStickyBar)) return;
        const combos = classCombinations(node.value) ?? [chunks.join(" ")];
        const rendered = combos.find(isHandRolledStickyBar);
        if (rendered !== undefined) check(node, rendered);
      },
    };
  },
};

// ui-kit/no-hand-rolled-field-label (#5589) — die Beschriftung eines Felds kommt
// aus `FieldLabel` oder der `label`-Prop. Fund: `<label>` mit text-xs/text-sm und
// font-medium/font-semibold, ein `<span>` dieser Art mit `block` und `mb-*` über
// dem Feld, oder als erstes Kind oder mit `block` in einem `<label>`. Kein Fund: Zeilen
// (Checkbox plus Text), `sr-only`, ein `<label>` in Knopfoptik (Datei wählen).
// Hard-zero seit #5590: die letzten Funde waren Gruppenköpfe und sind jetzt
// `SectionLabel`.
const FIELD_LABEL_SIZE = new Set(["text-xs", "text-sm"]);
const FIELD_LABEL_WEIGHT = new Set(["font-medium", "font-semibold"]);
const FIELD_LABEL_ROW_RE = /^(?:inline-)?(?:flex|grid)$/;
const FIELD_LABEL_CONTROL_LOOK_RE = /^(?:p[xy]?-|border|bg-|rounded|ring)/;

function isFieldLabelRow(utilities) {
  // Eine gestapelte Beschriftung (`flex flex-col`) ist keine Zeile.
  if (utilities.includes("flex-col") && !utilities.includes("flex-row")) {
    return false;
  }
  return utilities.some((token) => FIELD_LABEL_ROW_RE.test(token));
}

function classUtilities(text) {
  return text.split(/\s+/).filter(Boolean).map(bareUtility);
}

function parentJsxElement(openingElement) {
  const parent = openingElement.parent?.parent;
  return parent?.type === "JSXElement" ? parent.openingElement : null;
}

const noHandRolledFieldLabel = makeRatchetRule({
  ruleName: "no-hand-rolled-field-label",
  description:
    "Build field labels from FieldLabel or the label prop instead of styling a label or span by hand.",
  message:
    "Keine eigene Feldbeschriftung: `label`-Prop von Input, Textarea oder CustomSelect, sonst FieldLabel aus ~/components/ui/field-label. Immer normal geschrieben, nie in Versalien (#5589, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/field-label.tsx",
  detect: ({ classText: textOf, found }) => ({
    JSXOpeningElement(node) {
      const tag = jsxName(node.name);
      if (tag !== "label" && tag !== "span") return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      const utilities = classUtilities(textOf(className.value));
      if (
        !utilities.some((token) => FIELD_LABEL_SIZE.has(token)) ||
        !utilities.some((token) => FIELD_LABEL_WEIGHT.has(token)) ||
        utilities.includes("sr-only") ||
        isFieldLabelRow(utilities) ||
        utilities.some((token) => FIELD_LABEL_CONTROL_LOOK_RE.test(token))
      ) {
        return;
      }
      if (tag === "label") {
        found(node);
        return;
      }
      const isBlock = utilities.includes("block");
      if (isBlock && utilities.some((token) => token.startsWith("mb-"))) {
        found(node);
        return;
      }
      const parent = parentJsxElement(node);
      if (!parent || jsxName(parent.name) !== "label") return;
      const firstChild = node.parent.parent.children.find(
        (child) => child.type === "JSXElement",
      );
      if (!isBlock && firstChild !== node.parent) return;
      const parentClass = jsxAttribute(parent, "className");
      const parentUtilities = parentClass?.value
        ? classUtilities(textOf(parentClass.value))
        : [];
      if (!isFieldLabelRow(parentUtilities)) found(node);
    },
  }),
});

// ui-kit/no-hand-rolled-notice (#5591) — ein getönter Hinweiskasten kommt aus
// `Alert`. Fund: ein Block (`div`, `p`, Liste, `section`, `aside`, `details`)
// mit Rundung, Innenabstand, getöntem Rand `border-moto-<Ton>/N` und Fläche
// desselben Tons (`bg-moto-<Ton>-soft` oder `bg-moto-<Ton>/N`). Kein Fund:
// Pillen (`rounded-full`), Bedienelemente (Knopf, Link, Label), Inline-Text.
// Amber ist kein Ton von `Alert` (Warnungen sind orange): eine amber-Fläche
// mit Rundung und Innenabstand ist auch ohne Rand ein Fund. Hard-zero seit
// dem Abbau der Baseline.
const NOTICE_TAGS = new Set([
  "div",
  "p",
  "ul",
  "ol",
  "section",
  "aside",
  "details",
]);
const NOTICE_BORDER_RE = /^border-moto-([a-z]+)\/\d+$/;
const NOTICE_FILL_RE = /^bg-moto-([a-z]+)(?:-soft(?:\/\d+)?|\/\d+)$/;
const NOTICE_PADDING_RE = /^p[trblxyse]?-\d/;
const NOTICE_ROUNDED_RE = /^rounded(?:-(?:sm|md|lg|xl|2xl))?$/;

function isHandRolledNotice(utilities) {
  if (!utilities.some((token) => NOTICE_ROUNDED_RE.test(token))) return false;
  if (!utilities.some((token) => NOTICE_PADDING_RE.test(token))) return false;
  const borderHues = new Set(
    utilities.flatMap((token) => token.match(NOTICE_BORDER_RE)?.[1] ?? []),
  );
  return utilities.some((token) => {
    const hue = token.match(NOTICE_FILL_RE)?.[1];
    return hue !== undefined && (hue === "amber" || borderHues.has(hue));
  });
}

const noHandRolledNotice = makeRatchetRule({
  ruleName: "no-hand-rolled-notice",
  description:
    "Build tinted notice boxes from Alert instead of styling a bordered, tinted block by hand.",
  message:
    'Kein eigener Hinweiskasten: Alert aus ~/components/ui/alert (`type`, für Hinweise in Feldern `size="compact"`, Standortfarbe über `color`). Warnhinweise sind orange über `type="warning"` (#5591, .claude/rules/frontend-ui-kit.md).',
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/alert.tsx",
  detect: ({ classText: textOf, found }) => ({
    JSXOpeningElement(node) {
      if (!NOTICE_TAGS.has(jsxName(node.name))) return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      if (isHandRolledNotice(classUtilities(textOf(className.value)))) {
        found(node);
      }
    },
  }),
});

// ui-kit/no-arbitrary-slide-over-width — ein Slide-over nimmt seine Breite aus
// `size` (#5602). Freie Breiten waren acht verschiedene Werte für drei Größen.
const noArbitrarySlideOverWidth = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow free slide-over widths; SlideOverContent takes size md, lg or xl.",
    },
    messages: {
      arbitrary:
        'Keine freie Breite am Slide-over: SlideOverContent size="md", "lg" oder "xl" (#5602).',
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key) || key.startsWith(UI_KIT_DIR)) return {};
    return {
      JSXOpeningElement(node) {
        const isPanel = jsxName(node.name) === "SlideOverContent";
        for (const attribute of node.attributes) {
          const name = attribute.name?.name;
          if (!isPanel || (name !== "widthClass" && name !== "className")) {
            continue;
          }
          if (!attribute.value) continue;
          const text = classText(attribute.value, new Map());
          if (
            name === "widthClass"
              ? /\bsm:w-\[/.test(text)
              : /(?:^|[\s:])w-\[/.test(text)
          ) {
            context.report({ node: attribute, messageId: "arbitrary" });
          }
        }
      },
    };
  },
};

// ui-kit/no-slide-over-row-override — Kopf und Fuß des Slide-overs sind von sich
// aus Zeilen, der Fuß rechtsbündig (#5602). Wer `flex-row` nachträgt, baut die
// alte Fassung nach.
const noSlideOverRowOverride = makeKitClassRule({
  description:
    "SlideOverHeader and SlideOverFooter already are rows; do not add flex-row.",
  messageId: "rowOverride",
  message:
    "SlideOverHeader und SlideOverFooter sind schon Zeilen, der Fuß rechtsbündig: `flex-row` entfällt. Den Schließen-Knopf setzt der Kopf selbst, eigene Fassung über `close` (#5602).",
  detect: ({ name, tokens }) =>
    (name === "SlideOverHeader" || name === "SlideOverFooter") &&
    tokens.includes("flex-row"),
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
        const combinations = classCombinations(node, new Set(), sourceCode) ?? [
          chunks.join(" "),
        ];
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
          new Set(),
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

// ui-kit/no-own-spinner (#5588).

const SPIN_CLASS_RE = /(?:^|[\s"'`:])animate-spin(?:$|[\s"'`])/;
const ICON_PACKAGE_RE = /^(?:lucide-react|@phosphor-icons\/)/;
const SPINNER_ICON_RE =
  /^(?:Loader(?:2|Circle|Pinwheel)?(?:Icon)?|CircleNotch(?:Icon)?|Spinner(?:Gap|Ball)?(?:Icon)?)$/;

const noOwnSpinner = makeRatchetRule({
  ruleName: "no-own-spinner",
  description:
    "Waiting indicators come from Loading, SpinnerIcon or Button isLoading instead of an own spinner.",
  message:
    "Kein eigener Spinner: `Loading` für einen ganzen Bereich, `SpinnerIcon` neben Text oder im Symbolknopf, `Button isLoading loadingText=…` im Knopf (#5588, .claude/rules/frontend-ui-kit.md).",
  // Hard-zero: der Bestand ist abgebaut, jede neue Stelle schlägt fehl.
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/icons.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ found }) => ({
    ImportDeclaration(node) {
      if (!ICON_PACKAGE_RE.test(String(node.source.value))) return;
      for (const specifier of node.specifiers) {
        if (
          specifier.type === "ImportSpecifier" &&
          SPINNER_ICON_RE.test(specifier.imported.name)
        ) {
          found(specifier);
        }
      }
    },
    Literal(node) {
      if (typeof node.value === "string" && SPIN_CLASS_RE.test(node.value)) {
        found(node);
      }
    },
    TemplateElement(node) {
      if (SPIN_CLASS_RE.test(node.value.raw)) found(node);
    },
  }),
});

// Menüs und Popover (#5601): Rolle und schwebende Fläche gibt es nur im Kit,
// dort hängen Tastaturbedienung, Portal und Lage am Bildschirmrand.

const MENU_ROLE_RE =
  /^(?:menu|menuitem|menuitemradio|menuitemcheckbox|listbox|option)$/;
const HAND_POPOVER_CLASS_RE = /(?:^|[\s"'`])moto-popover-surface(?:$|[\s"'`])/;

/** Alle Zeichenketten eines Attributwerts, auch hinter `?:` und `&&`. */
function roleLiterals(node) {
  if (!node) return [];
  switch (node.type) {
    case "Literal":
      return typeof node.value === "string" ? [node.value] : [];
    case "JSXExpressionContainer":
      return roleLiterals(node.expression);
    case "ConditionalExpression":
      return [
        ...roleLiterals(node.consequent),
        ...roleLiterals(node.alternate),
      ];
    case "LogicalExpression":
      return [...roleLiterals(node.left), ...roleLiterals(node.right)];
    default:
      return [];
  }
}

const noHandRolledMenu = makeRatchetRule({
  ruleName: "no-hand-rolled-menu",
  description:
    "Menus and listboxes come from OverflowMenu, CustomSelect, ListboxDropdown or SuggestInput instead of own ARIA roles or a native datalist.",
  message:
    "Kein eigenes Menü: Aktionen über `OverflowMenu`, eine Auswahl über `CustomSelect` oder `ListboxDropdown`, Freitext mit Vorschlägen über `SuggestInput` (#5601, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/page-header/OverflowMenu.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ found }) => ({
    JSXAttribute(node) {
      if (node.name?.name !== "role") return;
      if (roleLiterals(node.value).some((role) => MENU_ROLE_RE.test(role))) {
        found(node);
      }
    },
    // Native Vorschlagsliste: eigene Optik je Browser, ohne Tastaturführung
    // des Kits und ohne iOS-Behandlung beim Tippen.
    JSXOpeningElement(node) {
      if (
        node.name?.type === "JSXIdentifier" &&
        node.name.name === "datalist"
      ) {
        found(node);
      }
    },
  }),
});

const noHandRolledPopover = makeRatchetRule({
  ruleName: "no-hand-rolled-popover",
  description:
    "Floating panels anchored to a control come from AnchoredPopover or OverflowMenu.",
  message:
    "Keine eigene schwebende Fläche: `AnchoredPopover` für einen Rahmen mit Inhalt, `OverflowMenu` für Aktionen (#5601, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/anchored-popover.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ found }) => ({
    Literal(node) {
      if (
        typeof node.value === "string" &&
        HAND_POPOVER_CLASS_RE.test(node.value)
      ) {
        found(node);
      }
    },
    TemplateElement(node) {
      if (HAND_POPOVER_CLASS_RE.test(node.value.raw)) found(node);
    },
  }),
});

// Listen-Bausteine (#5593): drei Muster, die es im Kit genau einmal gibt.

const ARBITRARY_SHADOW_RE = /(?:^|[\s:"'`!])shadow-[[(]/;

/** Fundstelle in jedem Klassen-Literal, auch in Template-Teilen. */
function classLiteralVisitor(found, matches) {
  return {
    Literal(node) {
      if (typeof node.value === "string" && matches(node.value)) found(node);
    },
    TemplateElement(node) {
      if (matches(node.value.raw)) found(node);
    },
  };
}

/**
 * Prüft den ganzen `className` eines Elements: auch über `cn("…", "…")`,
 * Template-Teile und Konstanten hinweg.
 */
function classNameVisitor({ classText, found }, matches) {
  return {
    JSXAttribute(node) {
      if (node.name?.name !== "className" || !node.value) return;
      if (matches(classText(node.value).split(/\s+/))) found(node);
    },
  };
}

const noArbitraryShadow = makeRatchetRule({
  ruleName: "no-arbitrary-shadow",
  description: "Shadows come from the scale, never from an arbitrary value.",
  message:
    "Kein Schatten mit freiem Wert: `shadow-sm`, `shadow-md` oder ein Kit-Baustein; ein Leuchtrand ist `ring-*` (#5593, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  // Die Signaturen der Portal-Navigation in dieser Datei sind keine Oberfläche.
  kitFile: "scripts/oxlint-plugin-ui-kit.mjs",
  exemptDir: UI_KIT_DIR,
  detect: ({ found }) =>
    classLiteralVisitor(found, (value) => ARBITRARY_SHADOW_RE.test(value)),
});

function isHandRolledListRowClass(tokens) {
  return (
    tokens.includes("bg-gray-50/50") &&
    tokens.includes("rounded-xl") &&
    tokens.some((token) => /^(?:px|p)-3$/.test(token)) &&
    !tokens.includes("border")
  );
}

const noHandRolledListRow = makeRatchetRule({
  ruleName: "no-hand-rolled-list-row",
  description: "List rows with their own surface come from ListRow.",
  message:
    "Keine eigene Listenzeile: `ListRow` (`href`, `onClick`, `tone`, `trailing`), beim Laden `SkeletonRows` (#5593, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/list-row.tsx",
  exemptDir: UI_KIT_DIR,
  detect: (tools) => classNameVisitor(tools, isHandRolledListRowClass),
});

// ui-kit/no-uppercase-label (#5590) — Gruppenköpfe und Zwischenüberschriften
// stehen normal geschrieben über `SectionLabel` (Epic #5576, Entscheidung 2).
// Fund: eine Klassenkette mit `uppercase` und `tracking-*` außerhalb des Kits,
// im `className` eines Elements (auch über Ternär-Zweige und `cn(…)`) oder in
// einer anderen Zeichenkette, etwa einer Konstante. Ausnahmen laut Epic: `/start`
// und die NFC-Druckanleitung; der Kicker des Elternportals kommt über das Kit
// (`SectionCard kicker`). Hard-zero seit dem Abbau der Baseline.
// Eltern-Ausnahmen aus #2619: Kinderkarten, Kinderwahl und Startseite des
// Elternportals behalten ihre Versalien. Nie anfassen, auch nicht für das Kit.
const UPPERCASE_LABEL_EXEMPT_FILES = [
  "src/app/start/page.tsx",
  "src/components/parent/child/child-day-card.tsx",
  "src/components/parent/child/child-switcher.tsx",
  "src/components/parent/start/parent-start-page.tsx",
];

const UPPERCASE_LABEL_EXEMPT_DIRS = ["src/app/help/nfc/"];

function isUppercaseLabelText(text) {
  const utilities = classUtilities(text);
  return (
    utilities.includes("uppercase") &&
    utilities.some((token) => token.startsWith("tracking-"))
  );
}

// A complete static chain is reported by the Literal/TemplateLiteral visitor.
// Recognise it through the same shallow constant lookup as classText() so the
// className visitor can report split cn(...) arguments without double-counting
// a direct `const label = "uppercase tracking-wide"` reference.
function containsStaticUppercaseLabelText(
  node,
  constants,
  seen = new WeakSet(),
  depth = 0,
) {
  if (!node || typeof node !== "object" || seen.has(node)) return false;
  seen.add(node);

  if (node.type === "Literal") {
    return typeof node.value === "string" && isUppercaseLabelText(node.value);
  }
  if (node.type === "TemplateLiteral") {
    return isUppercaseLabelText(
      node.quasis.map((quasi) => quasi.value.raw).join(" "),
    );
  }
  if (node.type === "Identifier" && depth < 2) {
    const init = constants.get(node.name);
    return init
      ? containsStaticUppercaseLabelText(init, constants, seen, depth + 1)
      : false;
  }

  for (const [key, value] of Object.entries(node)) {
    if (key === "parent" || key === "loc" || key === "range") continue;
    if (Array.isArray(value)) {
      if (
        value.some((child) =>
          containsStaticUppercaseLabelText(child, constants, seen, depth),
        )
      ) {
        return true;
      }
    } else if (
      containsStaticUppercaseLabelText(value, constants, seen, depth)
    ) {
      return true;
    }
  }
  return false;
}

function classNameUsesStaticUppercaseLabelConstant(value, constants) {
  let expression = value;
  while (
    expression?.type === "JSXExpressionContainer" ||
    expression?.type === "ChainExpression" ||
    expression?.type === "TSAsExpression" ||
    expression?.type === "TSTypeAssertion" ||
    expression?.type === "TypeCastExpression"
  ) {
    expression = expression.expression;
  }
  return (
    expression?.type === "Identifier" &&
    containsStaticUppercaseLabelText(expression, constants)
  );
}

function isClassNameAttribute(node) {
  return (
    node?.type === "JSXAttribute" &&
    node.name?.type === "JSXIdentifier" &&
    /(?:^c|C)lassName$/.test(node.name.name)
  );
}

function insideClassNameAttribute(node) {
  for (let current = node.parent; current; current = current.parent) {
    if (isClassNameAttribute(current)) return true;
    if (current.type === "JSXElement" || current.type === "Program") break;
  }
  return false;
}

const noUppercaseLabel = makeRatchetRule({
  ruleName: "no-uppercase-label",
  description:
    "Group headings come from SectionLabel; no `uppercase tracking-*` class chain outside the UI kit.",
  message:
    'Keine Versalien-Überschrift: Gruppenkopf oder Zwischenüberschrift über `SectionLabel` (`as`, `required`) aus ~/components/ui/section-label, Gruppenkopf einer Liste über `ListGroupHeader`, Mini-Tag über `StatusBadge size="xs"`, Tabellenkopf über `DataTable` (#5590, .claude/rules/frontend-ui-kit.md).',
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/section-label.tsx",
  exemptDir: UI_KIT_DIR,
  exemptDirs: UPPERCASE_LABEL_EXEMPT_DIRS,
  exemptFiles: UPPERCASE_LABEL_EXEMPT_FILES,
  onlySrc: true,
  detect: ({ constants, found }) => {
    const check = (node, text) => {
      if (insideClassNameAttribute(node)) return;
      if (isUppercaseLabelText(text)) found(node);
    };
    return {
      JSXAttribute(node) {
        if (!isClassNameAttribute(node) || !node.value) return;
        if (
          classTextAlternatives(node.value, constants).some(
            isUppercaseLabelText,
          ) &&
          !classNameUsesStaticUppercaseLabelConstant(node.value, constants)
        ) {
          found(node);
        }
      },
      Literal(node) {
        if (typeof node.value === "string") check(node, node.value);
      },
      TemplateLiteral(node) {
        check(node, node.quasis.map((quasi) => quasi.value.raw).join(" "));
      },
    };
  },
});

// ui-kit/no-hand-rolled-inset (#5592) — die graue Innenfläche kommt aus
// `InsetPanel`. Fund: ein Element mit Radius, `bg-gray-50` und Polster, auch über
// Ternär-Zweige und lokale Konstanten hinweg. Ein Knopf oder Link ist keine Fläche.
// Zweiter Fund: eine Flächen-Klassenkette in einer Modul-Konstante; dort entstand
// das nächste Mini-Kit. Hard-zero seit dem Abbau der Baseline.
// Eltern-Ausnahmen aus #2619: Kinderkarten, Kinderwahl und Startseite des
// Elternportals behalten ihre Flächen. Nie anfassen, auch nicht für das Kit.
const INSET_PARENT_EXCEPTIONS = [
  "src/components/parent/child/child-day-card.tsx",
  "src/components/parent/child/child-switcher.tsx",
  "src/components/parent/start/parent-start-page.tsx",
];
const INSET_ACTION_ELEMENTS = new Set([
  "a",
  "button",
  "Link",
  "NavigationLink",
]);
const SURFACE_RADIUS_RE = /(?:^|[\s])rounded-(?:md|lg|xl|2xl)(?=[\s]|$)/;
const SURFACE_LOOK_RE =
  /(?:^|[\s])(?:border|moto-content-surface|bg-(?:white|gray-\d+|moto-[\w-]+)(?:\/\d+)?)(?=[\s]|$)/;

function isHandRolledInsetClass(tokens) {
  return (
    tokens.some((token) => /^rounded-(?:md|lg|xl|2xl)$/.test(token)) &&
    tokens.some((token) => /^bg-gray-50(?:\/\d+)?$/.test(token)) &&
    tokens.some((token) => /^p[xytblrse]?-\d/.test(token))
  );
}

function isModuleLevel(declaration) {
  const holder = declaration.parent;
  return (
    holder?.type === "Program" ||
    (holder?.type === "ExportNamedDeclaration" &&
      holder.parent?.type === "Program")
  );
}

function staticStringValue(node) {
  if (node?.type === "TSAsExpression")
    return staticStringValue(node.expression);
  if (node?.type === "Literal") return node.value;
  if (node?.type === "TemplateLiteral") {
    return node.quasis.map((quasi) => quasi.value.raw).join(" ");
  }
  return null;
}

const noHandRolledInset = makeRatchetRule({
  ruleName: "no-hand-rolled-inset",
  description:
    "Inner surfaces come from InsetPanel; surface class chains do not live in module constants.",
  message:
    "Keine eigene Innenfläche: `InsetPanel` (`tone`, `framed`, `dashed`, `padding`, `as`) aus ~/components/ui/inset-panel, Flächenklassen nicht in einer Modul-Konstante (#5592, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/inset-panel.tsx",
  exemptDir: UI_KIT_DIR,
  exemptFiles: INSET_PARENT_EXCEPTIONS,
  // E2E-Dateien halten Selektoren als Konstanten, keine Oberfläche.
  onlySrc: true,
  detect: ({ classText, found }) => ({
    JSXOpeningElement(node) {
      if (INSET_ACTION_ELEMENTS.has(jsxName(node.name) ?? "")) return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      if (isHandRolledInsetClass(classText(className.value).split(/\s+/))) {
        found(node);
      }
    },
    VariableDeclaration(node) {
      if (!isModuleLevel(node)) return;
      for (const declarator of node.declarations) {
        const value = staticStringValue(declarator.init);
        if (
          typeof value === "string" &&
          SURFACE_RADIUS_RE.test(value) &&
          SURFACE_LOOK_RE.test(value)
        ) {
          found(declarator);
        }
      }
    },
  }),
});

function isHandRolledFramedListClass(tokens) {
  return (
    tokens.includes("divide-y") &&
    tokens.includes("border") &&
    tokens.some((token) => /^rounded-(?:md|lg|xl)$/.test(token))
  );
}

const noHandRolledFramedList = makeRatchetRule({
  ruleName: "no-hand-rolled-framed-list",
  description: "Framed lists with dividers come from FramedList.",
  message:
    "Keine eigene umrandete Liste: `FramedList` (`maxHeight`, `header`) (#5593, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/framed-list.tsx",
  exemptDir: UI_KIT_DIR,
  detect: (tools) => classNameVisitor(tools, isHandRolledFramedListClass),
});

const NAV_ROW_ELEMENTS = new Set(["a", "button", "Link", "NavigationLink"]);
const NAV_ROW_ARROWS = new Set([
  "CaretRightIcon",
  "ChevronRight",
  "ChevronRightIcon",
]);

/** Kinder einer Zeile, auch hinter `&&`, `?:` und einer lokalen Konstante. */
function navRowChildren(element, constants) {
  const direct = [...flatJsxChildren(element)];
  for (const child of element.children ?? []) {
    if (
      child.type === "JSXExpressionContainer" &&
      child.expression.type === "Identifier"
    ) {
      const init = constants.get(child.expression.name);
      if (init?.type === "JSXElement") direct.push(init);
    }
  }
  return direct;
}

function isNavRowArrow(element) {
  if (NAV_ROW_ARROWS.has(jsxName(element.openingElement.name))) return true;
  return element.children.some(
    (inner) => inner.type === "JSXText" && inner.value.trim() === "›",
  );
}

const noHandRolledNavRow = makeRatchetRule({
  ruleName: "no-hand-rolled-nav-row",
  description: "Rows that lead elsewhere with an arrow come from ListRowLink.",
  message:
    "Keine eigene Navigationszeile: `ListRowLink` (`icon`, `title`, `subtitle`, `badge`, Pfeil), einzeln mit `framed` (#5593, .claude/rules/frontend-ui-kit.md).",
  // Shrink-only: Auswahlzustand (DatabaseListItem), eigene Kante (Tagesplan) und
  // Ungelesen-Zeilen des Elternportals passen noch nicht in `ListRowLink`.
  baselines: {
    "no-hand-rolled-nav-row": {
      "src/components/database/database-list-item.tsx": 2,
      "src/components/parent/messages/group-threads-section.tsx": 1,
      "src/components/parent/messages/parent-messages-page.tsx": 1,
      "src/components/timetable/tagesplan-view.tsx": 1,
    },
  },
  allowlist: [],
  kitFile: "src/components/ui/list-row-link.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ classText, constants, found }) => ({
    JSXElement(node) {
      if (!NAV_ROW_ELEMENTS.has(jsxName(node.openingElement.name))) return;
      // Eine Zeile füllt die Breite; „Heute öffnen ›“ neben einem Titel nicht.
      const className = node.openingElement.attributes.find(
        (attr) =>
          attr.type === "JSXAttribute" && attr.name.name === "className",
      );
      if (!className?.value) return;
      const tokens = classText(className.value).split(/\s+/);
      if (!tokens.includes("w-full") && !tokens.includes("justify-between")) {
        return;
      }
      const arrow = navRowChildren(node, constants).find(isNavRowArrow);
      if (arrow) found(node);
    },
  }),
});

// Auswahl-Bausteine (#5595): Schalter, Kästchen, Feldgruppe und Auswahlzeile
// gibt es im Kit genau einmal. Hart null, ohne Baseline.

const TOGGLE_ROLES = new Set(["switch", "checkbox"]);
const PAINTED_BOX_SIZE_RE = /^(?:h-[45]|size-[45])$/;
const PAINTED_BOX_RADIUS_RE = /^rounded(?:-(?:sm|md))?$/;
const CHECK_ICONS = new Set(["Check", "CheckIcon"]);

function isPaintedCheckbox(element, classText) {
  const name = jsxName(element.openingElement.name);
  if (name !== "span" && name !== "div") return false;
  const className = jsxAttribute(element.openingElement, "className");
  if (!className?.value) return false;
  const tokens = classText(className.value).split(/\s+/);
  if (!tokens.some((token) => PAINTED_BOX_SIZE_RE.test(token))) return false;
  if (!tokens.some((token) => PAINTED_BOX_RADIUS_RE.test(token))) return false;
  if (!tokens.includes("border")) return false;
  return flatJsxChildren(element).some((child) =>
    CHECK_ICONS.has(jsxName(child.openingElement.name)),
  );
}

const noHandRolledToggleControl = makeRatchetRule({
  ruleName: "no-hand-rolled-toggle-control",
  description:
    "Switches and checkboxes come from Switch and Checkbox, not from role or a painted box.",
  message:
    "Kein eigener Schalter und kein gemaltes Kästchen: `Switch`/`SwitchRow` aus ~/components/ui/switch, `Checkbox` oder `CheckboxIndicator` aus ~/components/ui/checkbox (#5595, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/switch.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ classText, found }) => ({
    JSXElement(node) {
      // Auch `role={on ? "switch" : undefined}`: jeder Text im Ausdruck zählt.
      const roleAttribute = jsxAttribute(node.openingElement, "role");
      const roles = roleAttribute?.value
        ? classText(roleAttribute.value).split(/\s+/)
        : [];
      if (roles.some((role) => TOGGLE_ROLES.has(role))) {
        found(node.openingElement);
        return;
      }
      if (isPaintedCheckbox(node, classText)) found(node.openingElement);
    },
  }),
});

const FIELD_GROUP_RADIUS_RE = /^rounded(?:-(?:sm|md|lg|xl))?$/;

const noHandRolledFieldset = makeRatchetRule({
  ruleName: "no-hand-rolled-fieldset",
  description: "Framed field groups come from FormFieldset.",
  message:
    "Keine eigene umrandete Feldgruppe: `FormFieldset` (`legend`, `required`, `hint`, `error`) aus ~/components/ui/form-fieldset (#5595, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/form-fieldset.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ classText, found }) => ({
    JSXOpeningElement(node) {
      if (jsxName(node.name) !== "fieldset") return;
      const className = jsxAttribute(node, "className");
      if (!className?.value) return;
      const tokens = classText(className.value).split(/\s+/);
      if (
        tokens.includes("border") &&
        tokens.some((token) => FIELD_GROUP_RADIUS_RE.test(token))
      ) {
        found(node);
      }
    },
  }),
});

const CHOICE_CONTROLS = new Set(["Checkbox", "Radio"]);

function holdsChoiceControl(element) {
  return flatJsxChildren(element).some((child) => {
    const opening = child.openingElement;
    const name = jsxName(opening.name);
    if (CHOICE_CONTROLS.has(name)) return true;
    if (name === "input") {
      const type = staticAttributeValue(jsxAttribute(opening, "type"));
      if (type === "checkbox" || type === "radio") return true;
    }
    return holdsChoiceControl(child);
  });
}

const noHandRolledChoiceTile = makeRatchetRule({
  ruleName: "no-hand-rolled-choice-tile",
  description:
    "A framed row around a Checkbox or Radio comes from ChoiceTile or CheckboxCard.",
  message:
    "Keine eigene Auswahlzeile mit Rahmen: `ChoiceTile` (`selected`, `disabled`, `tone`) mit `Checkbox`/`Radio`, oder `CheckboxCard` (#5595, .claude/rules/frontend-ui-kit.md).",
  baselines: {},
  allowlist: [],
  kitFile: "src/components/ui/choice-tile.tsx",
  exemptDir: UI_KIT_DIR,
  detect: ({ classText, found }) => ({
    JSXElement(node) {
      if (jsxName(node.openingElement.name) !== "label") return;
      const className = jsxAttribute(node.openingElement, "className");
      if (!className?.value) return;
      const tokens = classText(className.value).split(/\s+/);
      const framed =
        tokens.some((token) => FIELD_GROUP_RADIUS_RE.test(token)) &&
        (tokens.includes("border") ||
          tokens.includes("moto-content-surface") ||
          tokens.includes("bg-white"));
      if (framed && holdsChoiceControl(node)) found(node.openingElement);
    },
  }),
});

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

// --- Kit-Varianten aus #5606 ---------------------------------------------------

const TABS_LOOK_RES = [
  /^bg-/,
  /^rounded(?:-|$)/,
  /^shadow(?:-|$)/,
  /^(?:min-)?h-/,
  /^text-/,
  /^font-/,
  /^flex-col$/,
];

function classTokens(attribute) {
  if (!attribute?.value) return [];
  return classText(attribute.value, new Map()).split(/\s+/).filter(Boolean);
}

const noTabsOverride = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Keep the look of TabsList and TabsTrigger in the kit; className carries layout only.",
    },
    messages: {
      override:
        'Keine eigene Optik an den Reitern ({{tokens}}): `variant` an TabsList wählen ("default", "line", "icon"), className nur für Layout (#5606).',
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (EXEMPT_FILE_RE.test(key) || key.includes(UI_KIT_DIR)) return {};
    return {
      JSXOpeningElement(node) {
        const name = jsxName(node.name);
        if (name !== "TabsList" && name !== "TabsTrigger") return;
        const tokens = classTokens(jsxAttribute(node, "className")).filter(
          (token) => TABS_LOOK_RES.some((re) => re.test(bareToken(token))),
        );
        if (tokens.length === 0) return;
        context.report({
          node,
          messageId: "override",
          data: { tokens: [...new Set(tokens)].join(" ") },
        });
      },
    };
  },
};

const TITLE_SIZE_RE = /^text-(?:xs|sm|base|lg|\d?xl|\[)/;

const noSectionCardTitleSize = {
  meta: {
    type: "problem",
    docs: {
      description: "SectionCard title sizes come from the titleSize prop.",
    },
    messages: {
      titleSize:
        "Keine Titelgröße über titleClassName ({{tokens}}): `titleSize` an SectionCard wählen (#5606).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (EXEMPT_FILE_RE.test(key) || key.includes(UI_KIT_DIR)) return {};
    return {
      JSXOpeningElement(node) {
        if (jsxName(node.name) !== "SectionCard") return;
        const tokens = classTokens(jsxAttribute(node, "titleClassName")).filter(
          (token) => TITLE_SIZE_RE.test(bareToken(token)),
        );
        if (tokens.length === 0) return;
        context.report({
          node,
          messageId: "titleSize",
          data: { tokens: tokens.join(" ") },
        });
      },
    };
  },
};

const noEmptyStateSurface = {
  meta: {
    type: "problem",
    docs: {
      description: "EmptyState carries no card classes of its own.",
    },
    messages: {
      surface:
        "Keine eigene Karte am Leerzustand: `empty` am Seitengerüst (TenantPage, ParentPage) oder EmptyState in SectionCard (#5606).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (EXEMPT_FILE_RE.test(key) || key.includes(UI_KIT_DIR)) return {};
    return {
      JSXOpeningElement(node) {
        if (jsxName(node.name) !== "EmptyState") return;
        const tokens = classTokens(jsxAttribute(node, "className"));
        if (tokens.includes("moto-content-surface")) {
          context.report({ node, messageId: "surface" });
        }
      },
    };
  },
};

const INITIALS_NAME_RE = /initial/i;

function rendersInitials(element) {
  return element.children.some((child) => {
    if (child.type !== "JSXExpressionContainer") return false;
    const expression = child.expression;
    const name =
      expression.type === "CallExpression"
        ? expression.callee?.name
        : expression.name;
    return typeof name === "string" && INITIALS_NAME_RE.test(name);
  });
}

const noHandRolledAvatar = {
  meta: {
    type: "problem",
    docs: {
      description: "Initials in a tinted circle come from Avatar.",
    },
    messages: {
      avatar:
        'Initialen im eigenen Kreis: Avatar aus ~/components/ui/avatar nehmen, grau mit tone="neutral" (#5606).',
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (EXEMPT_FILE_RE.test(key) || key.includes(UI_KIT_DIR)) return {};
    return {
      JSXElement(node) {
        const opening = node.openingElement;
        const name = jsxName(opening.name);
        if (!name || !/^[a-z]/.test(name)) return;
        const tokens = classTokens(jsxAttribute(opening, "className"));
        if (!tokens.includes("rounded-full")) return;
        if (!tokens.some((token) => bareToken(token).startsWith("bg-"))) return;
        if (rendersInitials(node)) {
          context.report({ node: opening, messageId: "avatar" });
        }
      },
    };
  },
};

// ui-kit/no-hand-rolled-empty-state (#5597). Gestrichelt und zentriert ist der
// Leer-Platzhalter; der kommt aus EmptyState. Die übrigen gestrichelten Flächen
// (Vorschau eines Upload-Felds, vorgemerkte Termine) sind nicht zentriert.
const noHandRolledEmptyState = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-built dashed empty states; use EmptyState variant=dashed.",
    },
    messages: {
      handRolled:
        'Ein gestrichelter Leerzustand kommt aus dem Kit: EmptyState variant="dashed" aus ~/components/ui/empty-state (#5597).',
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key) || key.startsWith("src/components/ui/")) {
      return {};
    }
    const sourceCode = context.sourceCode ?? context.getSourceCode?.();
    const DASHED = 1;
    const CENTERED = 2;
    const DASHED_AND_CENTERED = DASHED | CENTERED;
    const emptyStateClassPath = (text) => {
      const classes = text.split(/\s+/);
      return (
        (classes.includes("border-dashed") ? DASHED : 0) |
        (classes.includes("text-center") ? CENTERED : 0)
      );
    };
    const isDashedAndCentered = (text) =>
      emptyStateClassPath(text) === DASHED_AND_CENTERED;
    const combineClassPaths = (left, right) =>
      new Set(
        [...left].flatMap((leftPath) =>
          [...right].map((rightPath) => leftPath | rightPath),
        ),
      );
    const eitherClassPaths = (left, right) => new Set([...left, ...right]);
    const collectObjectClassPaths = (node) => {
      let paths = new Set([0]);
      const conditions = new Map();

      for (const property of node.properties) {
        if (property.type !== "Property" || property.computed) continue;
        const keyPaths = collectClassPaths(property.key);
        if (
          property.value.type === "Literal" ||
          (property.value.type === "Identifier" &&
            property.value.name === "undefined")
        ) {
          if (property.value.value) {
            paths = combineClassPaths(paths, keyPaths);
          }
          continue;
        }

        const negated =
          property.value.type === "UnaryExpression" &&
          property.value.operator === "!";
        const condition = negated ? property.value.argument : property.value;
        const conditionText = sourceCode?.getText(condition);
        if (conditionText == null || conditionText === "") continue;
        const variants = conditions.get(conditionText) ?? {
          whenFalse: new Set([0]),
          whenTrue: new Set([0]),
        };
        variants[negated ? "whenFalse" : "whenTrue"] = combineClassPaths(
          variants[negated ? "whenFalse" : "whenTrue"],
          keyPaths,
        );
        conditions.set(conditionText, variants);
      }

      for (const { whenFalse, whenTrue } of conditions.values()) {
        paths = combineClassPaths(paths, eitherClassPaths(whenFalse, whenTrue));
      }
      return paths;
    };
    const collectClassPaths = (node, seenVariables = new Set()) => {
      if (node == null || typeof node !== "object") return new Set([0]);
      if (node.type === "Literal" && typeof node.value === "string") {
        return new Set([emptyStateClassPath(node.value)]);
      }
      if (node.type === "TemplateElement") {
        return new Set([emptyStateClassPath(node.value.raw)]);
      }
      switch (node.type) {
        case "JSXExpressionContainer":
        case "ChainExpression":
        case "TSAsExpression":
        case "TSTypeAssertion":
        case "TypeCastExpression":
          return collectClassPaths(node.expression);
        case "Identifier": {
          const variable = findVariable(sourceCode, node);
          if (!variable || seenVariables.has(variable)) return new Set([0]);

          const nextSeenVariables = new Set(seenVariables);
          nextSeenVariables.add(variable);
          let paths = new Set();
          for (const definition of variable.defs) {
            if (
              definition.type !== "Variable" ||
              definition.node.type !== "VariableDeclarator" ||
              definition.parent?.kind !== "const" ||
              !definition.node.init
            ) {
              continue;
            }
            paths = eitherClassPaths(
              paths,
              collectClassPaths(definition.node.init, nextSeenVariables),
            );
          }
          return paths.size > 0 ? paths : new Set([0]);
        }
        case "ConditionalExpression":
          return eitherClassPaths(
            collectClassPaths(node.consequent),
            collectClassPaths(node.alternate),
          );
        case "LogicalExpression":
          return node.operator === "&&"
            ? eitherClassPaths(new Set([0]), collectClassPaths(node.right))
            : eitherClassPaths(
                collectClassPaths(node.left),
                collectClassPaths(node.right),
              );
        case "TemplateLiteral":
          return node.quasis.reduce(
            (paths, quasi, index) =>
              combineClassPaths(
                combineClassPaths(paths, collectClassPaths(quasi)),
                node.expressions[index] == null
                  ? new Set([0])
                  : collectClassPaths(node.expressions[index]),
              ),
            new Set([0]),
          );
        case "CallExpression":
          return node.arguments.reduce(
            (paths, argument) =>
              combineClassPaths(paths, collectClassPaths(argument)),
            new Set([0]),
          );
        case "ArrayExpression":
          return node.elements.reduce(
            (paths, element) =>
              combineClassPaths(paths, collectClassPaths(element)),
            new Set([0]),
          );
        case "SpreadElement":
        case "MemberExpression":
          return collectClassPaths(node.argument ?? node.object);
        case "TaggedTemplateExpression":
          return collectClassPaths(node.quasi);
        case "BinaryExpression":
          return node.operator === "+"
            ? combineClassPaths(
                collectClassPaths(node.left),
                collectClassPaths(node.right),
              )
            : new Set([0]);
        case "ObjectExpression":
          return collectObjectClassPaths(node);
        default:
          return new Set([0]);
      }
    };
    const belongsToClassNameAttribute = (node) => {
      let parent = node.parent;
      while (parent != null) {
        if (parent.type === "JSXAttribute") {
          return parent.name?.name === "className";
        }
        parent = parent.parent;
      }
      return false;
    };
    return {
      // Über alle Teile des className-Ausdrucks, auch Template und cn()-Argumente.
      JSXAttribute(node) {
        if (node.name?.name !== "className") return;
        if (
          [...collectClassPaths(node.value)].some(
            (path) => path === DASHED_AND_CENTERED,
          )
        ) {
          context.report({ node, messageId: "handRolled" });
        }
      },
      // Eine Klassenkette in einer Konstante.
      Literal(node) {
        if (
          typeof node.value === "string" &&
          !belongsToClassNameAttribute(node) &&
          isDashedAndCentered(node.value)
        ) {
          context.report({ node, messageId: "handRolled" });
        }
      },
      TemplateLiteral(node) {
        if (
          !belongsToClassNameAttribute(node) &&
          [...collectClassPaths(node)].some(
            (path) => path === DASHED_AND_CENTERED,
          )
        ) {
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
    "no-own-spinner": noOwnSpinner,
    "no-hand-rolled-dot": noHandRolledDot,
    "no-hand-rolled-dropzone": noHandRolledDropzone,
    "no-empty-state-surface": noEmptyStateSurface,
    "no-hand-rolled-avatar": noHandRolledAvatar,
    "no-hand-rolled-empty-state": noHandRolledEmptyState,
    "no-section-card-title-size": noSectionCardTitleSize,
    "no-tabs-override": noTabsOverride,
    "no-hand-rolled-icon-tile": noHandRolledIconTile,
    "no-hand-rolled-inset": noHandRolledInset,
    "no-hand-rolled-field-label": noHandRolledFieldLabel,
    "no-uppercase-label": noUppercaseLabel,
    "no-hand-rolled-fieldset": noHandRolledFieldset,
    "no-hand-rolled-choice-tile": noHandRolledChoiceTile,
    "no-hand-rolled-toggle-control": noHandRolledToggleControl,
    "no-hand-rolled-menu": noHandRolledMenu,
    "no-hand-rolled-notice": noHandRolledNotice,
    "no-hand-rolled-number-stepper": noHandRolledNumberStepper,
    "no-hand-rolled-overlay": noHandRolledOverlay,
    "no-hand-rolled-page-card": noHandRolledPageCard,
    "no-arbitrary-shadow": noArbitraryShadow,
    "no-hand-rolled-framed-list": noHandRolledFramedList,
    "no-hand-rolled-list-row": noHandRolledListRow,
    "no-hand-rolled-nav-row": noHandRolledNavRow,
    "no-hand-rolled-popover": noHandRolledPopover,
    "no-hand-rolled-pill": noHandRolledPill,
    "no-hand-rolled-portal-nav": noHandRolledPortalNav,
    "no-hand-rolled-removable-chip": noHandRolledRemovableChip,
    "no-arbitrary-slide-over-width": noArbitrarySlideOverWidth,
    "no-hand-rolled-sticky-bar": noHandRolledStickyBar,
    "no-slide-over-row-override": noSlideOverRowOverride,
    "no-hand-rolled-surface": noHandRolledSurface,
    "no-hand-rolled-weekday-toggle": noHandRolledWeekdayToggle,
    "no-hex-alpha-tint": noHexAlphaTint,
    "no-control-height-override": noControlHeightOverride,
    "no-important-override": noImportantOverride,
    "no-pill-dot": noPillDot,
    "no-raw-input": noRawInput,
    "no-raw-table": noRawTable,
    "no-em-dash-empty-value": noEmDashEmptyValue,
    "no-kit-domain-import": noKitDomainImport,
    "no-raw-z-index": noRawZIndex,
    "no-select-override": noSelectOverride,
    "no-rounded-3xl": noRounded3xl,
    "no-section-card-bleed": noSectionCardBleed,
    "no-status-dot-outside-plans": noStatusDotOutsidePlans,
    "no-shadcn-tokens": noShadcnTokens,
    "no-shadow-kit-name": noShadowKitName,
    "no-radix-outside-kit": noRadixOutsideKit,
    "no-tenant-kicker": noTenantKicker,
    "no-tabs-as-value-switcher": noTabsAsValueSwitcher,
    "no-tiny-text": noTinyText,
    "no-white-on-warm-fill": noWhiteOnWarmFill,
    "require-checkbox-label": requireCheckboxLabel,
  },
};
