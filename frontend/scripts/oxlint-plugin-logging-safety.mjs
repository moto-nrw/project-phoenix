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

function isLoggerType(identifier, sourceCode) {
  const type = identifier.typeAnnotation?.typeAnnotation;
  if (type?.type !== "TSTypeReference") return false;
  if (type.typeName.type !== "Identifier") return false;

  const variable = findVariable(sourceCode, type.typeName);
  if (!variable) return false;

  return variable.defs.some((definition) => {
    if (definition.type !== "ImportBinding") return false;
    if (definition.node.type !== "ImportSpecifier") return false;

    const importedName =
      definition.node.imported.name ?? definition.node.imported.value;
    return (
      importedName === "Logger" &&
      isLoggerModule(definition.parent.source.value)
    );
  });
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

  if (
    variable.identifiers.some((declaration) =>
      isLoggerType(declaration, sourceCode),
    )
  ) {
    return true;
  }

  return variable.defs.some(
    (definition) =>
      definition.type === "Variable" &&
      definition.node.type === "VariableDeclarator" &&
      isLoggerInitializer(definition.node.init, sourceCode, seenVariables),
  );
}

function isStructuredLoggerCall(node, sourceCode) {
  if (node?.type !== "CallExpression") return false;

  const callee = node.callee;
  return (
    callee.type === "MemberExpression" &&
    !callee.computed &&
    callee.object.type === "Identifier" &&
    isLoggerBinding(callee.object, sourceCode) &&
    callee.property.type === "Identifier" &&
    LOG_METHODS.has(callee.property.name)
  );
}

function isJsonStringifyCall(node) {
  const callee = node.callee;
  return (
    callee.type === "MemberExpression" &&
    !callee.computed &&
    callee.object.type === "Identifier" &&
    callee.object.name === "JSON" &&
    callee.property.type === "Identifier" &&
    callee.property.name === "stringify"
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
        if (!isJsonStringifyCall(node)) return;

        const ancestors = sourceCode.getAncestors(node);
        for (let index = ancestors.length - 1; index >= 0; index -= 1) {
          const ancestor = ancestors[index];
          if (isStructuredLoggerCall(ancestor, sourceCode)) {
            context.report({ node, messageId: "noSerializedContext" });
            return;
          }
        }
      },
    };
  },
};

export default {
  meta: { name: "logging-safety" },
  rules: { "no-serialized-context": noSerializedContext },
};
