import { existsSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  loadPhosphorEntryMaps,
  parseBarrel,
  rewritePhosphorImports,
  type PhosphorEntryMaps,
} from "./phosphor-imports-plugin";

const CSR_BARREL = `
import * as o from "./ssr/index.es.js";
import { Acorn as t, AcornIcon as n } from "./csr/Acorn.es.js";
import { PulseIcon as e9, Pulse as t9, PulseIcon as n9 } from "./csr/Pulse.es.js";
import { X as a, XIcon as b } from "./csr/X.es.js";
import { default as kho } from "./lib/IconBase.es.js";
import { IconContext as Aho } from "./lib/context.es.js";
export {
  t as Acorn,
  n as AcornIcon,
  e9 as ActivityIcon,
  kho as IconBase,
  Aho as IconContext,
  t9 as Pulse,
  n9 as PulseIcon,
  o as SSR,
  a as X,
  b as XIcon
};
`;

const SSR_BARREL = `
import { default as e } from "../lib/SSRBase.es.js";
import { Acorn as n, AcornIcon as c } from "./Acorn.es.js";
import { X as a, XIcon as b } from "./X.es.js";
export {
  n as Acorn,
  c as AcornIcon,
  e as SSRBase,
  a as X,
  b as XIcon
};
`;

const maps: PhosphorEntryMaps = {
  "@phosphor-icons/react": parseBarrel(CSR_BARREL),
  "@phosphor-icons/react/ssr": parseBarrel(SSR_BARREL),
};

// Built at runtime: this file runs through the same Vite plugin, which would
// otherwise rewrite the import statements inside these fixture strings.
const ROOT = ["@phosphor-icons", "react"].join("/");
const SSR = `${ROOT}/ssr`;

const csr = (file: string) => `@phosphor-icons/react/dist/csr/${file}`;

describe("parseBarrel", () => {
  it("maps every icon export to the module that defines it", () => {
    const map = parseBarrel(CSR_BARREL);

    expect(map.get("XIcon")).toEqual({ file: "X", name: "XIcon" });
    expect(map.get("X")).toEqual({ file: "X", name: "X" });
    expect(map.get("ActivityIcon")).toEqual({
      file: "Pulse",
      name: "PulseIcon",
    });
  });

  it("leaves library exports and the ssr namespace unmapped", () => {
    const map = parseBarrel(CSR_BARREL);

    expect(map.has("IconBase")).toBe(false);
    expect(map.has("IconContext")).toBe(false);
    expect(map.has("SSR")).toBe(false);
    expect(parseBarrel(SSR_BARREL).has("SSRBase")).toBe(false);
  });
});

describe("rewritePhosphorImports", () => {
  it("rewrites a named import to the per-icon module", () => {
    expect(
      rewritePhosphorImports(
        `import { XIcon } from "${ROOT}";\nuse(XIcon);`,
        maps,
      ),
    ).toBe(`import { XIcon } from "${csr("X")}";\nuse(XIcon);`);
  });

  it("splits several icons and keeps local aliases", () => {
    expect(
      rewritePhosphorImports(
        `import { AcornIcon, XIcon as Close } from '${ROOT}'`,
        maps,
      ),
    ).toBe(
      `import { AcornIcon } from "${csr("Acorn")}"; import { XIcon as Close } from "${csr("X")}";`,
    );
  });

  it("resolves barrel aliases to the defining module", () => {
    expect(
      rewritePhosphorImports(`import { ActivityIcon } from "${ROOT}";`, maps),
    ).toBe(`import { PulseIcon as ActivityIcon } from "${csr("Pulse")}";`);
  });

  it("keeps line breaks of a multi-line import", () => {
    const code = [
      "import {",
      "  AcornIcon, // tree",
      "  XIcon,",
      `} from "${ROOT}";`,
      "const line5 = 1;",
    ].join("\n");

    const out = rewritePhosphorImports(code, maps);

    expect(out).toBe(
      `import { AcornIcon } from "${csr("Acorn")}"; import { XIcon } from "${csr("X")}";\n\n\n\nconst line5 = 1;`,
    );
    expect(out?.split("\n")).toHaveLength(5);
  });

  it("keeps non-icon and unknown names on the barrel import", () => {
    expect(
      rewritePhosphorImports(
        `import { IconContext, XIcon, NotAnIcon } from "${ROOT}";`,
        maps,
      ),
    ).toBe(
      `import { XIcon } from "${csr("X")}"; import { IconContext, NotAnIcon } from "${ROOT}";`,
    );
  });

  it("turns remaining inline type specifiers into a type-only import", () => {
    expect(
      rewritePhosphorImports(
        `import { XIcon, type Icon as PhosphorIcon } from "${ROOT}";`,
        maps,
      ),
    ).toBe(
      `import { XIcon } from "${csr("X")}"; import type { Icon as PhosphorIcon } from "${ROOT}";`,
    );
  });

  it("keeps inline type specifiers next to remaining value imports", () => {
    expect(
      rewritePhosphorImports(
        `import { XIcon, IconContext, type Icon } from "${ROOT}";`,
        maps,
      ),
    ).toBe(
      `import { XIcon } from "${csr("X")}"; import { IconContext, type Icon } from "${ROOT}";`,
    );
  });

  it("rewrites the ssr entry to ssr modules", () => {
    expect(
      rewritePhosphorImports(`import { AcornIcon } from "${SSR}";`, maps),
    ).toBe(`import { AcornIcon } from "@phosphor-icons/react/dist/ssr/Acorn";`);
  });

  it("returns null when nothing can be rewritten", () => {
    for (const code of [
      `import type { Icon } from "${ROOT}";`,
      `import { type Icon, IconContext } from "${ROOT}";`,
      `import * as Icons from "${ROOT}";`,
      // Forms outside the named-import pattern stay on the barrel: slower,
      // but the module graph is the one the compiler would build anyway.
      `import Icons from "${ROOT}";`,
      `import Icons, { XIcon } from "${ROOT}";`,
      `export { XIcon } from "${ROOT}";`,
      `export * from "${ROOT}";`,
      `const icons = await import("${ROOT}");`,
      `import { "XIcon" as X } from "${ROOT}";`,
      `import { XIcon } from "@phosphor-icons/react/dist/csr/X";`,
      `import { XIcon } from "another-package";`,
    ]) {
      expect(rewritePhosphorImports(code, maps)).toBeNull();
    }
  });
});

describe("loadPhosphorEntryMaps", () => {
  it("maps installed icons to per-icon modules that exist", () => {
    const loaded = loadPhosphorEntryMaps();
    const dist = path.join(
      path.dirname(
        createRequire(import.meta.url).resolve(
          "@phosphor-icons/react/package.json",
        ),
      ),
      "dist",
    );

    for (const [entry, dir] of [
      ["@phosphor-icons/react", "csr"],
      ["@phosphor-icons/react/ssr", "ssr"],
    ] as const) {
      const map = loaded[entry];
      expect(map?.size).toBeGreaterThan(1000);
      expect(map?.get("XIcon")).toEqual({ file: "X", name: "XIcon" });
      const files = new Set([...(map?.values() ?? [])].map((m) => m.file));
      const missing = [...files].filter(
        (file) => !existsSync(path.join(dist, dir, `${file}.es.js`)),
      );
      expect(missing).toEqual([]);
    }
  });
});
