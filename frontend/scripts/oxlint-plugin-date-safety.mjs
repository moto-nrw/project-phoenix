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
      filename.includes("/src/test/")
    )
      return {};
    const file = filename.slice(filename.lastIndexOf("/src/") + 1);
    const remaining = [...(schoolTodayBaseline[file] ?? [])];
    const localTodayNames = new Set(["todayISO"]);
    const implicitDateFns = new Set([
      "isToday",
      "isYesterday",
      "isTomorrow",
      "isThisWeek",
      "isThisMonth",
      "isThisQuarter",
      "isThisYear",
      "startOfToday",
      "startOfYesterday",
      "startOfTomorrow",
      "endOfToday",
      "endOfYesterday",
      "endOfTomorrow",
    ]);
    const dateFnsNames = new Set();
    const dateFnsNamespaces = new Set();
    const memberName = (node) =>
      node.computed ? node.property.value : node.property.name;
    const isDateConstructor = (node) =>
      (node.type === "Identifier" && node.name === "Date") ||
      (node.type === "MemberExpression" &&
        node.object.type === "Identifier" &&
        node.object.name === "globalThis" &&
        memberName(node) === "Date");
    const isDateNow = (node) =>
      node?.type === "CallExpression" &&
      node.arguments.length === 0 &&
      node.callee.type === "MemberExpression" &&
      isDateConstructor(node.callee.object) &&
      memberName(node.callee) === "now";
    function check(node) {
      let source = context.sourceCode.lines[node.loc.start.line - 1].trim();
      if (file === "src/lib/date-helpers.ts") {
        let owner = node.parent;
        while (owner && owner.type !== "FunctionDeclaration")
          owner = owner.parent;
        source = `${owner?.id?.name ?? "<module>"}: ${source}`;
      }
      const index = remaining.indexOf(source);
      if (index >= 0) remaining.splice(index, 1);
      else context.report({ node, messageId: "browserToday" });
    }
    return {
      ImportDeclaration(node) {
        const source = node.source.value;
        if (source !== "date-fns" && !source.startsWith("date-fns/")) return;
        for (const specifier of node.specifiers) {
          if (specifier.type === "ImportNamespaceSpecifier")
            dateFnsNamespaces.add(specifier.local.name);
          else {
            const imported =
              specifier.type === "ImportSpecifier"
                ? specifier.imported.name
                : source.split("/").at(-1);
            if (implicitDateFns.has(imported))
              dateFnsNames.add(specifier.local.name);
          }
        }
      },
      ImportSpecifier(node) {
        if (node.imported.name === "todayISO")
          localTodayNames.add(node.local.name);
      },
      NewExpression(node) {
        if (
          isDateConstructor(node.callee) &&
          (node.arguments.length === 0 ||
            (node.arguments.length === 1 && isDateNow(node.arguments[0])))
        )
          check(node);
      },
      CallExpression(node) {
        if (
          (node.callee.type === "Identifier" &&
            (localTodayNames.has(node.callee.name) ||
              dateFnsNames.has(node.callee.name))) ||
          (node.callee.type === "MemberExpression" &&
            !node.callee.computed &&
            (memberName(node.callee) === "todayISO" ||
              (node.callee.object.type === "Identifier" &&
                dateFnsNamespaces.has(node.callee.object.name) &&
                implicitDateFns.has(memberName(node.callee)))))
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
