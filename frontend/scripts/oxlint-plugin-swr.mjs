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

// #5007: resources migrate incrementally; new hook-owned key literals may not grow.
const keyBaseline = JSON.parse(
  readFileSync(new URL("./swr-key-baseline.json", import.meta.url), "utf8"),
);
function isFreeKey(node) {
  if (!node) return false;
  if (node.type === "Literal" && typeof node.value === "string") return true;
  if (node.type === "TemplateLiteral") return true;
  return (
    node.type === "ConditionalExpression" &&
    (isFreeKey(node.consequent) || isFreeKey(node.alternate))
  );
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
    let actual = 0;
    return {
      CallExpression(node) {
        if (
          node.callee.type !== "Identifier" ||
          !["useSWRAuth", "useImmutableSWR"].includes(node.callee.name) ||
          !isFreeKey(node.arguments[0])
        )
          return;
        actual += 1;
        if (actual > expected)
          context.report({ node: node.arguments[0], messageId: "free" });
      },
      "Program:exit"(node) {
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

export default {
  meta: { name: "swr" },
  rules: { "no-direct-import": noDirectImport, "no-free-key": noFreeKey },
};
