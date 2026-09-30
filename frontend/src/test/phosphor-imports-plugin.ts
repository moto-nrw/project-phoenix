import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import type { Plugin } from "vite";

// Vitest counterpart of `experimental.optimizePackageImports` in
// next.config.js, which production builds already apply to
// "@phosphor-icons/react".
//
// Both entry points of the package are barrels: "@phosphor-icons/react"
// imports all ~1,500 csr icon modules plus the whole "/ssr" barrel, and
// "@phosphor-icons/react/ssr" imports all ~1,500 ssr icon modules. Vitest
// gives every test file a fresh worker (isolate: true), so each file paid
// ~0.9 s (root) or ~0.6 s (ssr) just to import icons it never renders.
//
// The plugin rewrites named imports in project sources and tests to the
// per-icon modules the package exports ("./dist/csr/*", "./dist/ssr/*"):
//
//   import { XIcon, InfoIcon as Info } from "@phosphor-icons/react";
//   →
//   import { XIcon } from "@phosphor-icons/react/dist/csr/X";
//   import { InfoIcon as Info } from "@phosphor-icons/react/dist/csr/Info";
//
// Which file holds which export comes from the installed barrel itself, so
// aliases such as `ActivityIcon` (defined in Pulse) resolve correctly. Type
// imports, non-icon exports (IconContext, IconBase, SSRBase) and unknown names
// stay on the original import. Line breaks are preserved so stack traces and
// coverage keep their line numbers.
//
// Limitation: `vi.mock("@phosphor-icons/react", …)` would no longer intercept
// the rewritten imports. Mock the per-icon module instead, or mock the
// component that renders the icon.

export interface IconModule {
  /** Per-icon module file name without extension, e.g. "Pulse". */
  readonly file: string;
  /** Export name inside that module, e.g. "PulseIcon". */
  readonly name: string;
}

type IconModuleMap = ReadonlyMap<string, IconModule>;

export type PhosphorEntryMaps = Readonly<Record<string, IconModuleMap>>;

const PACKAGE = "@phosphor-icons/react";

// Entry point → directory of its per-icon modules inside the package.
const ENTRY_DIRS = {
  [PACKAGE]: "csr",
  [`${PACKAGE}/ssr`]: "ssr",
} as const;

const IDENTIFIER = String.raw`[A-Za-z_$][\w$]*`;

/**
 * Maps every icon export of a phosphor barrel (`dist/index.es.js` or
 * `dist/ssr/index.es.js`) to the per-icon module that defines it.
 */
export function parseBarrel(source: string): Map<string, IconModule> {
  const locals = new Map<string, IconModule>();
  // Icon modules sit next to the barrel ("./X.es.js") or one level down
  // ("./csr/X.es.js"); "./lib/…" helpers are not icons and stay unmapped.
  const importRe =
    /import\s*\{([^}]*)\}\s*from\s*"\.\/(?:csr\/)?([A-Za-z0-9]+)\.es\.js"/g;
  for (const [, specifiers = "", file = ""] of source.matchAll(importRe)) {
    for (const spec of splitSpecifiers(specifiers)) {
      const parsed = parseSpecifier(spec);
      if (parsed) locals.set(parsed.local, { file, name: parsed.imported });
    }
  }

  const exports = new Map<string, IconModule>();
  for (const [, specifiers = ""] of source.matchAll(/export\s*\{([^}]*)\}/g)) {
    for (const spec of splitSpecifiers(specifiers)) {
      // `export { local as Exported }`: parseSpecifier reads the first name
      // as `imported` and the second as `local`.
      const parsed = parseSpecifier(spec);
      const target = parsed && locals.get(parsed.imported);
      if (parsed && target) exports.set(parsed.local, target);
    }
  }
  return exports;
}

/** Reads the installed barrels once per process. */
export function loadPhosphorEntryMaps(): PhosphorEntryMaps {
  const require = createRequire(import.meta.url);
  const dist = path.join(
    path.dirname(require.resolve(`${PACKAGE}/package.json`)),
    "dist",
  );
  const read = (file: string) =>
    parseBarrel(readFileSync(path.join(dist, file), "utf8"));
  return {
    [PACKAGE]: read("index.es.js"),
    [`${PACKAGE}/ssr`]: read("ssr/index.es.js"),
  };
}

const IMPORT_RE = new RegExp(
  String.raw`import\s*(type\s+)?\{([^}]*)\}\s*from\s*(["'])(${escape(PACKAGE)}(?:/ssr)?)\3[ \t]*;?`,
  "g",
);

/**
 * Rewrites named phosphor imports in `code` to per-icon modules. Returns
 * null when nothing changed.
 */
export function rewritePhosphorImports(
  code: string,
  maps: PhosphorEntryMaps,
): string | null {
  let changed = false;
  const out = code.replace(
    IMPORT_RE,
    (
      statement: string,
      typeOnly: string | undefined,
      specifierList: string,
      _quote: string,
      entry: keyof typeof ENTRY_DIRS,
    ) => {
      const map = maps[entry];
      if (typeOnly || !map) return statement;

      const rewritten: string[] = [];
      const keptValues: string[] = [];
      const keptTypes: string[] = [];
      for (const spec of splitSpecifiers(specifierList)) {
        const parsed = parseSpecifier(spec);
        // Unparseable syntax: leave the whole statement to the compiler.
        if (!parsed) return statement;
        const target = parsed.isType ? undefined : map.get(parsed.imported);
        if (target) {
          rewritten.push(
            `import { ${formatSpecifier(target.name, parsed.local)} } from "${PACKAGE}/dist/${ENTRY_DIRS[entry]}/${target.file}";`,
          );
        } else if (parsed.isType) {
          keptTypes.push(formatSpecifier(parsed.imported, parsed.local));
        } else {
          keptValues.push(formatSpecifier(parsed.imported, parsed.local));
        }
      }
      if (rewritten.length === 0) return statement;
      changed = true;

      if (keptValues.length > 0) {
        const types = keptTypes.map((spec) => `type ${spec}`);
        rewritten.push(
          `import { ${[...keptValues, ...types].join(", ")} } from "${entry}";`,
        );
      } else if (keptTypes.length > 0) {
        // `import { type X }` would survive verbatimModuleSyntax as a bare
        // side-effect import and load the barrel again.
        rewritten.push(
          `import type { ${keptTypes.join(", ")} } from "${entry}";`,
        );
      }
      const lineBreaks = statement.split("\n").length - 1;
      return rewritten.join(" ") + "\n".repeat(lineBreaks);
    },
  );
  return changed ? out : null;
}

/** Vite plugin applying {@link rewritePhosphorImports} to project files. */
export function phosphorPerIconImports(): Plugin {
  let maps: PhosphorEntryMaps | undefined;
  return {
    name: "moto:phosphor-per-icon-imports",
    enforce: "pre",
    transform(code, id) {
      if (id.includes("/node_modules/") || !code.includes(PACKAGE)) {
        return null;
      }
      if (!/\.[cm]?[jt]sx?$/.test(id.split("?")[0] ?? "")) return null;
      maps ??= loadPhosphorEntryMaps();
      const rewritten = rewritePhosphorImports(code, maps);
      // Line numbers are unchanged, so the existing source map stays valid.
      return rewritten === null ? null : { code: rewritten, map: null };
    },
  };
}

function splitSpecifiers(list: string): string[] {
  return list
    .replaceAll(/\/\*[\s\S]*?\*\/|\/\/[^\n]*/g, "")
    .split(",")
    .map((spec) => spec.trim())
    .filter(Boolean);
}

function parseSpecifier(
  spec: string,
): { imported: string; local: string; isType: boolean } | null {
  const match = new RegExp(
    String.raw`^(type\s+)?(${IDENTIFIER})(?:\s+as\s+(${IDENTIFIER}))?$`,
  ).exec(spec);
  if (!match?.[2]) return null;
  return {
    isType: Boolean(match[1]),
    imported: match[2],
    local: match[3] ?? match[2],
  };
}

function formatSpecifier(imported: string, local: string): string {
  return imported === local ? imported : `${imported} as ${local}`;
}

function escape(value: string): string {
  return value.replaceAll(/[.*+?^${}()|[\]\\/]/g, String.raw`\$&`);
}
