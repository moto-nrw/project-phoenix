import { existsSync, readFileSync } from "node:fs";
import { posix } from "node:path";

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

// #5007: code outside ~/lib/swr reaches the SWR cache only through the
// public modules of the directory, and those export nothing that bypasses
// the generation lock (see the PR section "Schreibwege in den SWR-Cache").
// Rules work on the resolved target of a module reference, not its text:
// `~/`, `@/`, relative paths, extensions and `index` name the same module.
const SWR_DIR = "src/lib/swr";
const PUBLIC_MODULES = new Set(
  [
    "",
    "hooks",
    "mutate",
    "config-boundaries",
    "scope-boundary",
    "portal-keys",
    "shell-keys",
    "parent-keys",
    "rooms-keys",
    "groups-keys",
    "staff-keys",
    "me-keys",
    "rooms",
    "groups",
    "staff",
    "me",
    "search-students-key",
    "room-derived-caches",
    // #5011: optimistic overlays; commits go through the locked writers.
    "optimistic",
  ].map((name) => (name ? `${SWR_DIR}/${name}` : SWR_DIR)),
);
const SOURCE_EXTENSIONS = [".ts", ".tsx", ".js", ".jsx", ".mjs"];

function isTestFile(file) {
  // Spec files count as tests, as for dependency-cruiser, Vitest and knip.
  return /\.(test|spec)\.[jt]sx?$/.test(file) || file.startsWith("src/test/");
}

/** Root of the checked tree (the directory that holds `src/`) and the file. */
function filePaths(context) {
  const file = String(
    context.physicalFilename ?? context.filename ?? "",
  ).replaceAll("\\", "/");
  const index = file.lastIndexOf("/src/");
  return { root: index === -1 ? null : file.slice(0, index), file };
}

/**
 * The module a specifier names, relative to the tree root and without
 * extension or `/index`; null for packages and other trees.
 */
function resolveModule(paths, specifier) {
  if (typeof specifier !== "string" || paths.root === null) return null;
  let target;
  if (specifier.startsWith("~/") || specifier.startsWith("@/")) {
    target = posix.join(paths.root, "src", specifier.slice(2));
  } else if (specifier.startsWith(".")) {
    target = posix.resolve(posix.dirname(paths.file), specifier);
  } else return null;
  let key = posix.relative(paths.root, target);
  for (const extension of SOURCE_EXTENSIONS) {
    if (key.endsWith(extension)) key = key.slice(0, -extension.length);
  }
  return key.replace(/\/index$/, "");
}

function isSWRModule(key) {
  return key === SWR_DIR || key?.startsWith(`${SWR_DIR}/`);
}

// The names a module of this repository exports, read from its source.
const exportCache = new Map();
function exportedNames(key) {
  if (exportCache.has(key)) return exportCache.get(key);
  const names = new Set();
  exportCache.set(key, names);
  const base = new URL(`../${key}`, import.meta.url).pathname;
  const candidates = [
    ...SOURCE_EXTENSIONS.map((extension) => `${base}${extension}`),
    ...SOURCE_EXTENSIONS.map((extension) => `${base}/index${extension}`),
  ];
  const path = candidates.find((candidate) => existsSync(candidate));
  if (!path) return names;
  const source = readFileSync(path, "utf8");
  const declared =
    /export\s+(?:declare\s+)?(?:default\s+)?(?:async\s+)?(?:function\*?|const|let|var|class|type|interface|enum)\s+([A-Za-z_$][\w$]*)/g;
  for (const match of source.matchAll(declared)) names.add(match[1]);
  if (/export\s+default\b/.test(source)) names.add("default");
  for (const match of source.matchAll(/export\s+(?:type\s+)?\{([^}]*)\}/g)) {
    for (const part of match[1].split(",")) {
      const name = part
        .replace(/^\s*type\s+/, "")
        .split(/\s+as\s+/)
        .pop()
        ?.trim();
      if (name) names.add(name);
    }
  }
  for (const match of source.matchAll(
    /export\s+\*\s+from\s+["']([^"']+)["']/g,
  )) {
    const target = resolveModule(
      {
        root: new URL("..", import.meta.url).pathname.replace(/\/$/, ""),
        file: path,
      },
      match[1],
    );
    if (target) for (const name of exportedNames(target)) names.add(name);
  }
  return names;
}

function importedName(specifier) {
  const imported = specifier.imported ?? specifier.local;
  return imported?.name ?? imported?.value;
}

const noCacheAccess = {
  meta: {
    type: "problem",
    schema: [],
    messages: {
      internal:
        "{{target}} is internal to ~/lib/swr and bypasses the generation lock. Use the public modules (~/lib/swr, its key and factory modules).",
      unknown:
        "{{target}} exports no '{{name}}'. Outside ~/lib/swr only the public surface of the module is reachable.",
      opaque:
        "A computed member of a ~/lib/swr namespace cannot be checked against its exports.",
    },
  },
  create(context) {
    const paths = filePaths(context);
    const file = fileKey(context);
    if (file.startsWith(`${SWR_DIR}/`) || isTestFile(file)) return {};
    const namespaces = new Map();

    function target(node, sourceNode) {
      const key = resolveModule(paths, packageName(sourceNode));
      if (!isSWRModule(key)) return null;
      if (!PUBLIC_MODULES.has(key)) {
        context.report({ node, messageId: "internal", data: { target: key } });
        return null;
      }
      return key;
    }
    function checkName(node, key, name) {
      if (name && !exportedNames(key).has(name))
        context.report({
          node,
          messageId: "unknown",
          data: { target: key, name },
        });
    }

    return {
      ImportDeclaration(node) {
        const key = target(node, node.source);
        if (!key) return;
        for (const specifier of node.specifiers) {
          if (specifier.type === "ImportNamespaceSpecifier")
            namespaces.set(specifier.local.name, key);
          else if (specifier.type === "ImportDefaultSpecifier")
            checkName(specifier, key, "default");
          else checkName(specifier, key, importedName(specifier));
        }
      },
      ExportNamedDeclaration(node) {
        if (!node.source) return;
        const key = target(node, node.source);
        if (!key) return;
        for (const specifier of node.specifiers)
          checkName(
            specifier,
            key,
            specifier.local?.name ?? specifier.local?.value,
          );
      },
      ExportAllDeclaration(node) {
        target(node, node.source);
      },
      ImportExpression(node) {
        target(node, node.source);
      },
      TSImportType(node) {
        const key = target(node, node.source ?? node.argument);
        const name = node.qualifier?.name;
        if (key && name) checkName(node, key, name);
      },
      TSExternalModuleReference(node) {
        target(node, node.expression);
      },
      CallExpression(node) {
        if (node.callee.type === "Identifier" && node.callee.name === "require")
          target(node, node.arguments[0]);
      },
      VariableDeclarator(node) {
        // const { SWRConfig } = Lib
        if (
          node.id.type !== "ObjectPattern" ||
          node.init?.type !== "Identifier"
        )
          return;
        const key = namespaces.get(node.init.name);
        if (!key) return;
        for (const property of node.id.properties) {
          if (property.type !== "Property" || property.computed)
            context.report({ node: property, messageId: "opaque" });
          else
            checkName(property, key, property.key.name ?? property.key.value);
        }
      },
      MemberExpression(node) {
        if (node.object.type !== "Identifier") return;
        const key = namespaces.get(node.object.name);
        if (!key) return;
        if (node.computed) {
          context.report({ node, messageId: "opaque" });
          return;
        }
        checkName(node, key, node.property.name);
      },
    };
  },
};

// #5007: resources migrate incrementally; new hook-owned keys outside the
// ~/lib/swr factories may not grow. A factory key is built by a member of a
// `*Keys` object or another export of a ~/lib/swr module, directly or through
// a local constant; `null` and branches of factory keys count too. Anything
// else at the call is free: literals, templates, joins, other modules'
// constants and parameters. The keyed hooks count under any local alias and
// as members of a ~/lib/swr namespace.
const keyBaseline = JSON.parse(
  readFileSync(new URL("./swr-key-baseline.json", import.meta.url), "utf8"),
);
const KEYED_HOOKS = new Set(["useSWRAuth", "useImmutableSWR", "useSWRWithId"]);

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
    const paths = filePaths(context);
    const file = fileKey(context);
    if (file.startsWith("src/lib/swr/") || /\.test\.[jt]sx?$/.test(file))
      return {};
    const expected = keyBaseline[file] ?? 0;
    const factories = new Set();
    const hooks = new Set(KEYED_HOOKS);
    const namespaces = new Set();
    const locals = new Map();
    const aliases = [];
    const calls = [];
    // Whether `node` names a keyed hook: an import under any alias, a
    // namespace member, or a local bound to one of those.
    const isHook = (node) =>
      (node?.type === "Identifier" && hooks.has(node.name)) ||
      (node?.type === "MemberExpression" &&
        !node.computed &&
        node.object.type === "Identifier" &&
        namespaces.has(node.object.name) &&
        KEYED_HOOKS.has(node.property.name));
    return {
      ImportDeclaration(node) {
        if (!isSWRModule(resolveModule(paths, packageName(node.source))))
          return;
        for (const specifier of node.specifiers) {
          factories.add(specifier.local.name);
          if (specifier.type === "ImportNamespaceSpecifier")
            namespaces.add(specifier.local.name);
          else if (KEYED_HOOKS.has(importedName(specifier)))
            hooks.add(specifier.local.name);
        }
      },
      VariableDeclarator(node) {
        if (!node.init) return;
        if (node.id.type === "Identifier") {
          locals.set(node.id.name, node.init);
          aliases.push([node.id.name, node.init]);
        } else if (
          node.id.type === "ObjectPattern" &&
          node.init.type === "Identifier"
        ) {
          // const { useSWRAuth: h } = Lib
          for (const property of node.id.properties) {
            const name = property.key?.name ?? property.key?.value;
            if (
              property.type === "Property" &&
              property.value?.type === "Identifier" &&
              KEYED_HOOKS.has(name)
            )
              aliases.push([
                property.value.name,
                {
                  type: "MemberExpression",
                  computed: false,
                  object: node.init,
                  property: { type: "Identifier", name },
                },
              ]);
          }
        }
      },
      CallExpression(node) {
        if (node.arguments[0]) calls.push(node);
      },
      // Locals and aliases may be declared after the call that uses them.
      "Program:exit"(node) {
        for (let changed = true; changed;) {
          changed = false;
          for (const [name, init] of aliases) {
            if (!hooks.has(name) && isHook(init)) {
              hooks.add(name);
              changed = true;
            }
          }
        }
        const keys = calls
          .filter((call) => isHook(call.callee))
          .map((call) => call.arguments[0]);
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

// #5007: test support (src/test/**, *.test.* and *.spec.* modules,
// ~/lib/swr/testing) re-exports SWR's provider and other doubles.
// Production code reaches it by no path; the target is resolved like every
// module reference above.
const noTestSupportImport = {
  meta: {
    type: "problem",
    schema: [],
    messages: {
      support:
        "{{target}} is test support. Production code must not import it, also not through a re-export.",
    },
  },
  create(context) {
    const paths = filePaths(context);
    const file = fileKey(context);
    if (isTestFile(file) || /\.stories\.[jt]sx?$/.test(file)) return {};
    function check(node, sourceNode) {
      const key = resolveModule(paths, packageName(sourceNode));
      // src/test/**, and every test or spec module wherever it lives.
      if (
        key === "src/test" ||
        key?.startsWith("src/test/") ||
        /\.(test|spec)$/.test(key ?? "")
      )
        context.report({ node, messageId: "support", data: { target: key } });
    }
    return {
      ImportDeclaration(node) {
        check(node, node.source);
      },
      ExportNamedDeclaration(node) {
        if (node.source) check(node, node.source);
      },
      ExportAllDeclaration(node) {
        check(node, node.source);
      },
      ImportExpression(node) {
        check(node, node.source);
      },
      TSImportType(node) {
        check(node, node.source ?? node.argument);
      },
      CallExpression(node) {
        if (node.callee.type === "Identifier" && node.callee.name === "require")
          check(node, node.arguments[0]);
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
    "no-test-support-import": noTestSupportImport,
  },
};
