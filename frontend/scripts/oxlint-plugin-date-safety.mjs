import schoolTodayBaseline from "./school-today-baseline.mjs";

const noUtcDateExtraction = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow deriving a calendar date from toISOString(); it yields the UTC date, one day behind Berlin between 00:00 and 02:00.",
    },
    messages: {
      noUtcDateExtraction:
        "Do not derive a calendar date via .toISOString().{{method}}(). Use toISODate()/todayISO()/parseISODate() from ~/lib/date-helpers.",
    },
    schema: [],
  },
  create(context) {
    return {
      CallExpression(node) {
        const callee = node.callee;
        if (callee.type !== "MemberExpression" || callee.computed) return;
        if (callee.property.type !== "Identifier") return;
        const method = callee.property.name;
        if (!["split", "slice", "substring", "substr"].includes(method)) return;
        const obj = callee.object;
        if (
          obj.type === "CallExpression" &&
          obj.callee.type === "MemberExpression" &&
          !obj.callee.computed &&
          obj.callee.property.type === "Identifier" &&
          obj.callee.property.name === "toISOString"
        ) {
          context.report({
            node,
            messageId: "noUtcDateExtraction",
            data: { method },
          });
        }
      },
    };
  },
};

// Flag the source, not a particular formatter: this also catches aliases,
// intermediate variables, local getters and date-fns compositions.
const noBrowserToday = {
  meta: {
    type: "problem",
    schema: [],
    messages: {
      browserToday:
        "School today must come from berlinTodayISO()/useBerlinToday(). Browser-local dates and instants need a reviewed shrink-only baseline entry.",
      staleBaseline: "Remove the stale school-today baseline entry: {{source}}",
    },
  },
  create(context) {
    const filename = context.filename.replaceAll("\\", "/");
    if (
      !filename.includes("/src/") ||
      /\.(test|spec|stories)\.[cm]?[jt]sx?$/.test(filename) ||
      filename.includes("/src/test/") ||
      filename.endsWith("/src/lib/date-helpers.ts")
    )
      return {};
    const file = filename.slice(filename.lastIndexOf("/src/") + 1);
    const remaining = [...(schoolTodayBaseline[file] ?? [])];
    const localTodayNames = new Set(["todayISO"]);
    function check(node) {
      const source = context.sourceCode.lines[node.loc.start.line - 1].trim();
      const index = remaining.indexOf(source);
      if (index >= 0) remaining.splice(index, 1);
      else context.report({ node, messageId: "browserToday" });
    }
    return {
      ImportSpecifier(node) {
        if (node.imported.name === "todayISO")
          localTodayNames.add(node.local.name);
      },
      NewExpression(node) {
        if (
          node.callee.type === "Identifier" &&
          node.callee.name === "Date" &&
          node.arguments.length === 0
        )
          check(node);
      },
      CallExpression(node) {
        if (
          (node.callee.type === "Identifier" &&
            localTodayNames.has(node.callee.name)) ||
          (node.callee.type === "MemberExpression" &&
            !node.callee.computed &&
            node.callee.property.name === "todayISO")
        )
          check(node);
      },
      "Program:exit"(node) {
        for (const source of remaining)
          context.report({
            node,
            messageId: "staleBaseline",
            data: { source },
          });
      },
    };
  },
};

export default {
  meta: { name: "date-safety" },
  rules: {
    "no-utc-date-extraction": noUtcDateExtraction,
    "no-browser-today": noBrowserToday,
  },
};
