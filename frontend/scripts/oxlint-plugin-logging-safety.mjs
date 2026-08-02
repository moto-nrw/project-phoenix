const LOG_METHODS = new Set(["debug", "info", "warn", "error"]);

const noSerializedContext = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow JSON.stringify inside structured logger calls because serialized fields bypass key-based redaction.",
    },
    messages: {
      noSerializedContext:
        "Do not serialize values inside logger calls. Pass explicit, non-sensitive structured fields instead.",
    },
    schema: [],
  },
  create(context) {
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    return {
      CallExpression(node) {
        const callee = node.callee;
        if (callee.type !== "MemberExpression" || callee.computed) return;
        if (callee.object.type !== "Identifier") return;
        if (!/logger$/i.test(callee.object.name)) return;
        if (callee.property.type !== "Identifier") return;
        if (!LOG_METHODS.has(callee.property.name)) return;

        if (/\bJSON\s*\.\s*stringify\s*\(/.test(sourceCode.getText(node))) {
          context.report({ node, messageId: "noSerializedContext" });
        }
      },
    };
  },
};

export default {
  meta: { name: "logging-safety" },
  rules: { "no-serialized-context": noSerializedContext },
};
