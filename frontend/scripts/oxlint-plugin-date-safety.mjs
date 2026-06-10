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

export default {
  meta: { name: "date-safety" },
  rules: { "no-utc-date-extraction": noUtcDateExtraction },
};
