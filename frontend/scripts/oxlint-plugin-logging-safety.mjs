const LOG_METHODS = new Set(["debug", "info", "warn", "error"]);

function findVariable(sourceCode, identifier) {
  let scope = sourceCode.getScope(identifier);

  while (scope) {
    const variable = scope.set.get(identifier.name);
    if (variable) return variable;
    scope = scope.upper;
  }

  return undefined;
}

function isLoggerModule(source) {
  return typeof source === "string" && /(?:^|\/)logger$/.test(source);
}

function isCreateLoggerFactory(identifier, sourceCode) {
  const variable = findVariable(sourceCode, identifier);
  if (!variable) return false;

  return variable.defs.some((definition) => {
    if (definition.type !== "ImportBinding") return false;
    if (definition.node.type !== "ImportSpecifier") return false;

    const importedName =
      definition.node.imported.name ?? definition.node.imported.value;
    return (
      importedName === "createLogger" &&
      isLoggerModule(definition.parent.source.value)
    );
  });
}

function isCreateLoggerCall(node, sourceCode) {
  return (
    node?.type === "CallExpression" &&
    node.callee.type === "Identifier" &&
    isCreateLoggerFactory(node.callee, sourceCode)
  );
}

function isLoggerInitializer(node, sourceCode, seenVariables) {
  if (isCreateLoggerCall(node, sourceCode)) return true;
  if (node?.type !== "CallExpression") return false;

  if (
    node.callee.type === "MemberExpression" &&
    !node.callee.computed &&
    node.callee.object.type === "Identifier" &&
    node.callee.property.type === "Identifier" &&
    node.callee.property.name === "child"
  ) {
    return isLoggerBinding(node.callee.object, sourceCode, seenVariables);
  }

  if (
    node.callee.type === "Identifier" &&
    node.callee.name === "useMemo" &&
    node.arguments[0]?.type === "ArrowFunctionExpression"
  ) {
    return isLoggerInitializer(
      node.arguments[0].body,
      sourceCode,
      seenVariables,
    );
  }

  return false;
}

function isLoggerBinding(identifier, sourceCode, seenVariables = new Set()) {
  const variable = findVariable(sourceCode, identifier);
  if (!variable || seenVariables.has(variable)) return false;
  seenVariables.add(variable);

  return variable.defs.some(
    (definition) =>
      definition.type === "Variable" &&
      definition.node.type === "VariableDeclarator" &&
      isLoggerInitializer(definition.node.init, sourceCode, seenVariables),
  );
}

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
        if (!isLoggerBinding(callee.object, sourceCode)) return;
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
