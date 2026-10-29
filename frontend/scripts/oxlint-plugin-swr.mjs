import { existsSync, readFileSync } from "node:fs";

// #5002: direct SWR package references bypass the shared hooks/configuration.
// Snapshot only today's files, including tests and type imports. Counts may
// only shrink: new references fail, removed references require a smaller entry.
const baselineUrl = new URL("./swr-import-baseline.json", import.meta.url);
const baseline = JSON.parse(readFileSync(baselineUrl, "utf8"));

// A deleted file has no Program visitor. Keep its exception from surviving
// silently, including when oxlint runs on a subset of the tree.
for (const file of Object.keys(baseline)) {
  if (!existsSync(new URL(`../${file}`, import.meta.url))) {
    throw new Error(
      `Shrink scripts/swr-import-baseline.json: remove deleted file ${file}.`,
    );
  }
}

function fileKey(context) {
  const file = String(
    context.physicalFilename ?? context.filename ?? "",
  ).replaceAll("\\", "/");
  const index = file.lastIndexOf("/src/");
  return index === -1 ? file : file.slice(index + 1);
}

function packageName(node) {
  if (node?.type === "Literal") return node.value;
  if (node?.type === "TSLiteralType") return packageName(node.literal);
  if (node?.type === "TemplateLiteral" && node.expressions.length === 0) {
    return node.quasis[0]?.value.cooked;
  }
  return null;
}

const noDirectImport = {
  meta: {
    type: "problem",
    docs: {
      description: "Import SWR through ~/lib/swr; shrink legacy exceptions.",
    },
    schema: [],
    messages: {
      direct:
        "Import SWR through ~/lib/swr, not '{{source}}'. Existing exceptions in scripts/swr-import-baseline.json are shrink-only.",
      stale:
        "Shrink scripts/swr-import-baseline.json for {{file}}: {{actual}} direct SWR references remain (baseline: {{expected}}). Remove the entry when none remain.",
    },
  },
  create(context) {
    const file = fileKey(context);
    // This module implements the boundary, including its boundary-level tests.
    if (file.startsWith("src/lib/swr/")) return {};
    const expected = baseline[file] ?? 0;
    let actual = 0;

    function check(node, sourceNode) {
      const source = packageName(sourceNode);
      if (
        typeof source !== "string" ||
        (source !== "swr" && !source.startsWith("swr/"))
      ) {
        return;
      }
      actual += 1;
      if (actual > expected) {
        context.report({ node, messageId: "direct", data: { source } });
      }
    }

    return {
      ImportDeclaration(node) {
        check(node, node.source);
      },
      ExportNamedDeclaration(node) {
        check(node, node.source);
      },
      ExportAllDeclaration(node) {
        check(node, node.source);
      },
      ImportExpression(node) {
        check(node, node.source);
      },
      TSImportType(node) {
        check(node, node.source);
      },
      TSExternalModuleReference(node) {
        check(node, node.expression);
      },
      CallExpression(node) {
        if (
          node.callee.type === "Identifier" &&
          node.callee.name === "require"
        ) {
          check(node, node.arguments[0]);
        }
      },
      "Program:exit"(node) {
        if (actual < expected) {
          context.report({
            node,
            messageId: "stale",
            data: { file, actual, expected },
          });
        }
      },
    };
  },
};

// #5007: resources migrate incrementally; new hook-owned keys outside the
// ~/lib/swr factories may not grow. A factory key is built by a member of a
// `*Keys` object or another export of a ~/lib/swr module, directly or through
// a local constant; `null` and branches of factory keys count too. Anything
// else at the call is free: literals, templates, joins, other modules'
// constants and parameters.
const keyBaseline = JSON.parse(
  readFileSync(new URL("./swr-key-baseline.json", import.meta.url), "utf8"),
);
const KEYED_HOOKS = new Set(["useSWRAuth", "useImmutableSWR", "useSWRWithId"]);
const SWR_MODULE = /^~\/lib\/swr(\/|$)/;

function rootIdentifier(node) {
  let current = node;
  while (current?.type === "MemberExpression") current = current.object;
  return current?.type === "Identifier" ? current.name : null;
}

function isFactoryKey(node, factories, locals, seen = new Set()) {
  if (!node) return false;
  switch (node.type) {
    case "Literal":
      return node.value === null;
    case "TSAsExpression":
    case "TSNonNullExpression":
    case "TSSatisfiesExpression":
    case "ParenthesizedExpression":
      return isFactoryKey(node.expression, factories, locals, seen);
    case "ConditionalExpression":
      return (
        isFactoryKey(node.consequent, factories, locals, seen) &&
        isFactoryKey(node.alternate, factories, locals, seen)
      );
    case "CallExpression":
      return factories.has(rootIdentifier(node.callee));
    case "MemberExpression":
      return factories.has(rootIdentifier(node));
    case "Identifier": {
      if (factories.has(node.name)) return true;
      const init = locals.get(node.name);
      if (!init || seen.has(node.name)) return false;
      seen.add(node.name);
      return isFactoryKey(init, factories, locals, seen);
    }
    default:
      return false;
  }
}

const noFreeKey = {
  meta: {
    type: "problem",
    schema: [],
    messages: {
      free: "Build resource keys in ~/lib/swr factories, not at the hook call. scripts/swr-key-baseline.json is shrink-only.",
      stale:
        "Shrink scripts/swr-key-baseline.json for {{file}}: {{actual}} free keys remain (baseline: {{expected}}).",
    },
  },
  create(context) {
    const file = fileKey(context);
    if (file.startsWith("src/lib/swr/") || /\.test\.[jt]sx?$/.test(file))
      return {};
    const expected = keyBaseline[file] ?? 0;
    const factories = new Set();
    const locals = new Map();
    const keys = [];
    return {
      ImportDeclaration(node) {
        if (!SWR_MODULE.test(String(node.source.value))) return;
        for (const specifier of node.specifiers) {
          if (specifier.type !== "ImportNamespaceSpecifier")
            factories.add(specifier.local.name);
        }
      },
      VariableDeclarator(node) {
        if (node.id.type === "Identifier" && node.init)
          locals.set(node.id.name, node.init);
      },
      CallExpression(node) {
        if (
          node.callee.type === "Identifier" &&
          KEYED_HOOKS.has(node.callee.name) &&
          node.arguments[0]
        )
          keys.push(node.arguments[0]);
      },
      // Locals may be declared after the hook call that reads them.
      "Program:exit"(node) {
        let actual = 0;
        for (const key of keys) {
          if (isFactoryKey(key, factories, locals)) continue;
          actual += 1;
          if (actual > expected)
            context.report({ node: key, messageId: "free" });
        }
        if (actual < expected)
          context.report({
            node,
            messageId: "stale",
            data: { file, actual, expected },
          });
      },
    };
  },
};

// #5007: the cache is reachable only through the generation-locked hooks of
// ~/lib/swr. A raw `useSWRConfig` write or read, or a stamp through the
// generation internals, would bypass the lock (G1, G4, G8 of #5182). Hard
// zero outside ~/lib/swr; tests may drive the internals.
const SWR_INTERNALS = /^~\/lib\/swr\/generation$/;
const noCacheAccess = {
  meta: {
    type: "problem",
    schema: [],
    messages: {
      config:
        "useSWRConfig bypasses the generation lock. Use useInvalidateMatching, useInvalidateFamily, useTenantMutate or the seed helpers of ~/lib/swr.",
      internals:
        "~/lib/swr/generation is internal. Seed through stampSWRSeeds, writeSWRSeeds or useSWRSeedFrom of ~/lib/swr.",
    },
  },
  create(context) {
    const file = fileKey(context);
    if (file.startsWith("src/lib/swr/") || /\.test\.[jt]sx?$/.test(file))
      return {};
    function check(node, source, names) {
      if (typeof source !== "string") return;
      if (SWR_INTERNALS.test(source)) {
        context.report({ node, messageId: "internals" });
        return;
      }
      if (
        (SWR_MODULE.test(source) || source === "swr") &&
        names.includes("useSWRConfig")
      )
        context.report({ node, messageId: "config" });
    }
    function importedNames(node) {
      return (node.specifiers ?? []).map((specifier) =>
        specifier.type === "ImportSpecifier"
          ? (specifier.imported.name ?? specifier.imported.value)
          : specifier.type === "ExportSpecifier"
            ? (specifier.local.name ?? specifier.local.value)
            : null,
      );
    }
    return {
      ImportDeclaration(node) {
        check(node, packageName(node.source), importedNames(node));
      },
      ExportNamedDeclaration(node) {
        check(node, packageName(node.source), importedNames(node));
      },
      ExportAllDeclaration(node) {
        check(node, packageName(node.source), []);
      },
      ImportExpression(node) {
        check(node, packageName(node.source), []);
      },
    };
  },
};

export default {
  meta: { name: "swr" },
  rules: {
    "no-direct-import": noDirectImport,
    "no-free-key": noFreeKey,
    "no-cache-access": noCacheAccess,
  },
};
