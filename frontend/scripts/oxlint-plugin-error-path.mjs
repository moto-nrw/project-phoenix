// Error display path (ADR 0006 Punkt 4, issue #2520,
// .claude/rules/error-path.md).
//
// A failure reaches the user on one path: an `ApiError` shown through
// `useApiErrorDisplay` (toast), `useApiFormError` (`FormErrorAlert` plus the
// field) or `useApiLoadError` (`LoadErrorAlert` / the `TenantPage` error).
// The text comes from the error catalog for the code, never from the error
// object itself. Five hard-zero rules, no exception list; production files
// under src/ in all portals (tests, stories, mocks and type declarations are
// out of scope).
//
// What counts as the path is named by construct, not by a list of
// tolerated files: PATH_MODULES_RE names the three modules that implement
// it (form-error-alert, tenant-page, toast), TOAST_PATH_MODULE_RE the toast
// queue in ToastContext, KIT_DIR_RE the UI kit that renders field errors at
// the control.
//
// How errors are recognised, honestly: there is no type information in a
// lint rule. A caught value is exact (a `catch (e)` parameter or the
// parameter of a `.catch((e) => …)` handler). Every other error object is
// recognised by its NAME (`e`, `ex`, `err`, `error`, `cause`, `exception`,
// `…Error`, `…Err`, or a member named so); error text by the names
// `errorText` / `errorMessage` / `errorMsg` / `…ErrorText` and by being
// copied from an error's `message`. A value named otherwise slips through;
// review covers that gap.
//
//   error-path/no-error-alert — `<Alert>` from ui/alert whose `type` is or
//                               can be "error", outside PATH_MODULES. A
//                               `type` the rule cannot resolve to string
//                               literals (a call, a prop, an object field)
//                               is reported as well, because it can be
//                               "error". A hint that is not a failure of
//                               the user's action uses `warning` or `info`;
//                               a form failure goes through
//                               `FormErrorAlert`, a load failure through
//                               `LoadErrorAlert`.
//
//   error-path/no-error-text-display — an error's text reaching a display
//                               sink. The text is `x.message` of an error
//                               object (unless the display path handed the
//                               value out: `useApiFormError`,
//                               `useApiLoadError`, `useSwrLoadError`,
//                               `useFormError`, `fieldErrorProps` carry
//                               catalog text), or a caught value through
//                               `String(e)`, `${e}` or `e.toString()`.
//                               Sinks: a React setter (`setX(…)`),
//                               `toast.*`, `show…`, `notify…`, `invalid`,
//                               `alert`, `t(…)`, or JSX. The rule follows
//                               the text through conditionals, templates,
//                               object literals, `const` copies and
//                               same-file helpers that return it. A caught
//                               value itself is reported when handed to a
//                               toast / `notify…` / `alert` / `t(…)` or
//                               rendered as a JSX child or as `message` /
//                               `title` / `description` / `label`; the path
//                               hooks (`show…`) and setters may take it.
//                               Logger calls are diagnostics and pass.
//
//   error-path/no-hand-rolled-alert — `role="alert"` (also as a branch of
//                               a conditional) on a hand-written element
//                               outside the UI kit (src/components/ui/).
//                               Field errors go through the kit control's
//                               `error` prop, form and load errors through
//                               the path components.
//
//   error-path/no-raw-error-toast — an error toast by hand outside
//                               src/contexts/ToastContext.tsx: `.error(…)`
//                               on the `useToast()` API (bound to it or
//                               named `toast`), or `error` destructured from
//                               `useToast()`. A failed API call goes through
//                               `useApiErrorDisplay().show(error, …)`, a
//                               failure the screen found itself through
//                               `useApiErrorDisplay().showMessage(text)`.
//                               `toast.warning` stays: a hint, not a
//                               failure.
//
//   error-path/no-error-text-match — a program decision on an error's text:
//                               a string search method (see MATCH_METHODS)
//                               on it, a RegExp `.test` / `.exec` with it
//                               as argument, or an equality comparison with
//                               a string. Branch on `ApiError.code` or the
//                               HTTP status instead.

// The modules that implement the display path. They render the error Alert
// on behalf of every caller.
const PATH_MODULES_RE =
  /(?:^|\/)src\/components\/ui\/(?:form-error-alert|tenant-page|toast)\.tsx$/;

const OUT_OF_SCOPE_RE =
  /(?:\.(?:test|spec|stories)\.[cm]?[jt]sx?$)|(?:\.d\.ts$)|(?:\/__tests__\/)|(?:\/src\/test\/)|(?:\/__mocks__\/)/;

const ALERT_MODULE_RE = /(?:^|\/)alert$/;

const ERROR_NAME_RE =
  /^(?:e|ex|err|error|cause|exception|[A-Za-z]\w*(?:Error|Err))$/;
const ERROR_TEXT_NAME_RE =
  /^(?:(?:error|err)(?:Text|Message|Msg)|[a-z]\w*(?:Error|Err)(?:Text|Message|Msg))$/;

const DISPLAY_CALLEE_RE =
  /^(?:set[A-Z]\w*|show[A-Z]?\w*|notify\w*|invalid|alert|t)$/;
const TOAST_OBJECT_RE = /toast|notif/i;
// Producers of the display path: what they hand back is catalog text.
const PATH_VALUE_CALLEE_RE =
  /^(?:useApiFormError|useApiLoadError|useSwrLoadError|useFormError|fieldErrorProps)$/;
const LOG_OBJECT_RE = /^(?:logger|log|\w*Logger|console|Sentry)$/;

const MATCH_METHODS = new Set([
  "includes",
  "startsWith",
  "endsWith",
  "match",
  "matchAll",
  "search",
  "indexOf",
  "lastIndexOf",
]);
const STRING_PASS_THROUGH = new Set([
  "trim",
  "trimStart",
  "trimEnd",
  "toLowerCase",
  "toUpperCase",
  "toLocaleLowerCase",
  "toLocaleUpperCase",
  "slice",
  "substring",
  "replace",
  "replaceAll",
  "normalize",
]);

function fileName(context) {
  return String(context.physicalFilename ?? context.filename ?? "").replaceAll(
    "\\",
    "/",
  );
}

function inScope(context) {
  const name = fileName(context);
  return /\/src\/.*\.[cm]?[jt]sx?$/.test(name) && !OUT_OF_SCOPE_RE.test(name);
}

function findVariable(sourceCode, identifier) {
  let scope = sourceCode.getScope(identifier);
  while (scope) {
    const variable = scope.set.get(identifier.name);
    if (variable) return variable;
    scope = scope.upper;
  }
  return undefined;
}

function unwrap(node) {
  let current = node;
  while (
    current &&
    (current.type === "TSAsExpression" ||
      current.type === "TSNonNullExpression" ||
      current.type === "TSSatisfiesExpression" ||
      current.type === "TSTypeAssertion" ||
      current.type === "ParenthesizedExpression" ||
      current.type === "ChainExpression")
  ) {
    current = current.expression;
  }
  return current;
}

function propertyName(member) {
  if (member.type !== "MemberExpression") return null;
  if (!member.computed && member.property.type === "Identifier") {
    return member.property.name;
  }
  if (member.computed && member.property.type === "Literal") {
    return String(member.property.value);
  }
  return null;
}

function isCatchParameter(identifier, sourceCode) {
  const variable = findVariable(sourceCode, identifier);
  return Boolean(variable?.defs.some((def) => def.type === "CatchClause"));
}

/** A caught value, by construct rather than by name: a `catch (e)`
 *  parameter or the parameter of a `.catch((e) => …)` handler. */
function isCaughtValue(node, sourceCode) {
  const target = unwrap(node);
  if (target?.type !== "Identifier") return false;
  const variable = findVariable(sourceCode, target);
  return Boolean(
    variable?.defs.some((def) => {
      if (def.type === "CatchClause") return true;
      if (def.type !== "Parameter") return false;
      const fn = def.node;
      const call = fn.parent;
      return (
        call?.type === "CallExpression" &&
        call.arguments[0] === fn &&
        fn.params[0] === def.name &&
        unwrap(call.callee)?.type === "MemberExpression" &&
        propertyName(unwrap(call.callee)) === "catch"
      );
    }),
  );
}

/** The binding was filled by the display path itself (`useApiFormError()`,
 *  `useApiLoadError()`, `useSwrLoadError(…)`, `fieldErrorProps(…)`), directly
 *  or by destructuring. Its `message` is already the catalog text. */
function isPathValue(identifier, sourceCode, depth = 0) {
  const variable = findVariable(sourceCode, identifier);
  if (!variable || depth > 4) return false;
  return variable.defs.some((def) => {
    if (def.type !== "Variable" || def.node.type !== "VariableDeclarator") {
      return false;
    }
    const init = unwrap(def.node.init);
    // const { error } = form; const detail = form.error;
    const root = rootIdentifier(init);
    if (root) return isPathValue(root, sourceCode, depth + 1);
    if (init?.type !== "CallExpression") return false;
    const { name } = calleeParts(init.callee);
    return name !== null && PATH_VALUE_CALLEE_RE.test(name);
  });
}

function rootIdentifier(node) {
  let current = unwrap(node);
  while (current?.type === "MemberExpression") current = unwrap(current.object);
  return current?.type === "Identifier" ? current : null;
}

/** `x` in `x.message` is an error object: a catch parameter, a name that
 *  says so, or a member named so (`query.error`) — unless the value comes
 *  from the display path. */
function isErrorObject(node, sourceCode) {
  const object = unwrap(node);
  if (!object) return false;
  if (object.type === "Identifier") {
    if (isCatchParameter(object, sourceCode)) return true;
    return ERROR_NAME_RE.test(object.name) && !isPathValue(object, sourceCode);
  }
  if (object.type === "MemberExpression") {
    const name = propertyName(object);
    if (name === null || !ERROR_NAME_RE.test(name)) return false;
    const root = rootIdentifier(object);
    return !(root && isPathValue(root, sourceCode));
  }
  return false;
}

function isErrorMessageMember(node, sourceCode) {
  const member = unwrap(node);
  return (
    member?.type === "MemberExpression" &&
    propertyName(member) === "message" &&
    isErrorObject(member.object, sourceCode)
  );
}

function calleeParts(callee) {
  const target = unwrap(callee);
  if (target?.type === "Identifier") return { object: null, name: target.name };
  if (target?.type === "MemberExpression") {
    const object = unwrap(target.object);
    let objectName = null;
    if (object?.type === "Identifier") objectName = object.name;
    else if (object?.type === "MemberExpression") {
      objectName = propertyName(object);
    }
    return { object: objectName, name: propertyName(target) };
  }
  return { object: null, name: null };
}

function isLoggerCall(call) {
  const { object, name } = calleeParts(call.callee);
  if (object && LOG_OBJECT_RE.test(object)) return true;
  return name !== null && /^(?:log|logError|captureException)$/.test(name);
}

function isDisplayCall(call) {
  const { object, name } = calleeParts(call.callee);
  if (object && TOAST_OBJECT_RE.test(object)) return true;
  if (object === null && name !== null && /^toast/i.test(name)) return true;
  if (object && LOG_OBJECT_RE.test(object)) return false;
  return name !== null && DISPLAY_CALLEE_RE.test(name);
}

// Calls that show their argument as text. Unlike `show…` (the path hooks
// take the error itself) and setters (state may hold an error object for
// the path), handing them an error object displays its text.
function isTextDisplayCall(call) {
  const { object, name } = calleeParts(call.callee);
  if (object && TOAST_OBJECT_RE.test(object)) return true;
  if (object === null && name !== null && /^toast/i.test(name)) return true;
  if (object && LOG_OBJECT_RE.test(object)) return false;
  return name !== null && /^(?:notify\w*|alert|t)$/.test(name);
}

// JSX attributes that a component renders as visible text.
const TEXT_ATTRIBUTES = new Set(["message", "title", "description", "label"]);

function isFunction(node) {
  return (
    node?.type === "ArrowFunctionExpression" ||
    node?.type === "FunctionExpression" ||
    node?.type === "FunctionDeclaration"
  );
}

/** The binding a function is reachable by in this file, if any. */
function functionBinding(fn, sourceCode) {
  if (fn.type === "FunctionDeclaration" && fn.id) {
    return findVariable(sourceCode, fn.id);
  }
  let holder = fn.parent;
  // useCallback(() => …, []) and memo wrappers keep the binding one level up.
  if (holder?.type === "CallExpression") holder = holder.parent;
  if (
    holder?.type === "VariableDeclarator" &&
    holder.id.type === "Identifier"
  ) {
    return findVariable(sourceCode, holder.id);
  }
  return undefined;
}

function readReferences(variable) {
  return (variable?.references ?? [])
    .filter((reference) => reference.isRead?.() ?? true)
    .map((reference) => reference.identifier)
    .filter((identifier) => identifier !== variable.identifiers[0]);
}

/**
 * Follows an error's own text upwards from `start` and calls `onDisplay` at
 * the first display sink it reaches. Copies (`const m = err.message`) and
 * same-file helpers that return the text are followed too.
 */
function traceDisplay(start, sourceCode, onDisplay) {
  const seen = new Set();
  const work = [start];

  while (work.length > 0) {
    const origin = work.pop();
    if (seen.has(origin)) continue;
    seen.add(origin);

    let child = origin;
    let node = origin.parent;
    while (node) {
      const type = node.type;
      if (
        type === "ConditionalExpression" ||
        type === "LogicalExpression" ||
        type === "TemplateLiteral" ||
        type === "BinaryExpression" ||
        type === "ArrayExpression" ||
        type === "ObjectExpression" ||
        type === "SpreadElement" ||
        type === "AwaitExpression" ||
        type === "TSAsExpression" ||
        type === "TSNonNullExpression" ||
        type === "TSSatisfiesExpression" ||
        type === "ParenthesizedExpression" ||
        type === "ChainExpression" ||
        type === "SequenceExpression"
      ) {
        if (type === "ConditionalExpression" && child === node.test) break;
        child = node;
        node = node.parent;
        continue;
      }
      if (type === "Property") {
        if (child !== node.value) break;
        child = node;
        node = node.parent;
        continue;
      }
      if (type === "MemberExpression") {
        // err.message.trim() keeps being the text; any other member does not.
        if (
          child === node.object &&
          STRING_PASS_THROUGH.has(propertyName(node) ?? "") &&
          node.parent?.type === "CallExpression" &&
          node.parent.callee === node
        ) {
          child = node.parent;
          node = node.parent.parent;
          continue;
        }
        break;
      }
      if (type === "CallExpression") {
        if (child === node.callee) break;
        if (isLoggerCall(node)) break;
        if (isDisplayCall(node)) onDisplay(node);
        break;
      }
      if (type === "JSXExpressionContainer") {
        onDisplay(node);
        break;
      }
      if (type === "VariableDeclarator") {
        if (child === node.init && node.id.type === "Identifier") {
          const variable = findVariable(sourceCode, node.id);
          for (const reference of readReferences(variable)) {
            work.push(reference);
          }
        }
        break;
      }
      if (type === "AssignmentExpression") {
        if (child === node.right && node.left.type === "Identifier") {
          const variable = findVariable(sourceCode, node.left);
          for (const reference of readReferences(variable)) {
            if (reference !== node.left) work.push(reference);
          }
        }
        break;
      }
      if (type === "ReturnStatement" || isFunction(node)) {
        // Returned from a function: arrow body or return statement.
        let fn = node;
        while (fn && !isFunction(fn)) fn = fn.parent;
        if (!fn) break;
        if (type === "ReturnStatement" || child === fn.body) {
          if (
            fn.parent?.type === "CallExpression" &&
            isDisplayCall(fn.parent)
          ) {
            onDisplay(fn.parent);
            break;
          }
          const binding = functionBinding(fn, sourceCode);
          for (const reference of readReferences(binding)) {
            const call = reference.parent;
            if (call?.type === "CallExpression" && call.callee === reference) {
              work.push(call);
            }
          }
        }
        break;
      }
      break;
    }
  }
}

/** Values copied from an error's text, so `const msg = err.message;
 *  msg.includes(…)` is seen as a match on the error text. */
function isErrorText(node, sourceCode, depth = 0) {
  const target = unwrap(node);
  if (!target || depth > 4) return false;
  if (isErrorMessageMember(target, sourceCode)) return true;
  if (target.type === "Identifier") {
    if (ERROR_TEXT_NAME_RE.test(target.name)) return true;
    const variable = findVariable(sourceCode, target);
    return Boolean(
      variable?.defs.some(
        (def) =>
          def.type === "Variable" &&
          def.node.type === "VariableDeclarator" &&
          def.node.init &&
          containsErrorText(def.node.init, sourceCode, depth + 1),
      ),
    );
  }
  if (target.type === "CallExpression") {
    // err.message.toLowerCase().includes(…)
    const callee = unwrap(target.callee);
    if (
      callee?.type === "MemberExpression" &&
      STRING_PASS_THROUGH.has(propertyName(callee) ?? "")
    ) {
      return isErrorText(callee.object, sourceCode, depth + 1);
    }
  }
  return false;
}

function containsErrorText(node, sourceCode, depth) {
  const target = unwrap(node);
  if (!target) return false;
  if (target.type === "ConditionalExpression") {
    return (
      containsErrorText(target.consequent, sourceCode, depth) ||
      containsErrorText(target.alternate, sourceCode, depth)
    );
  }
  if (target.type === "LogicalExpression") {
    return (
      containsErrorText(target.left, sourceCode, depth) ||
      containsErrorText(target.right, sourceCode, depth)
    );
  }
  return isErrorText(target, sourceCode, depth);
}

// --- error-path/no-error-alert ----------------------------------------------

function isAlertElement(nameNode, sourceCode) {
  if (nameNode?.type !== "JSXIdentifier") return false;
  const variable = findVariable(sourceCode, nameNode);
  if (!variable) return false;
  return variable.defs.some((def) => {
    if (def.type !== "ImportBinding") return false;
    if (def.node.type !== "ImportSpecifier") return false;
    const imported = def.node.imported.name ?? def.node.imported.value;
    return (
      imported === "Alert" && ALERT_MODULE_RE.test(def.parent.source.value)
    );
  });
}

function literalUnion(typeNode) {
  if (!typeNode) return null;
  if (
    typeNode.type === "TSLiteralType" &&
    typeof typeNode.literal.value === "string"
  ) {
    return [typeNode.literal.value];
  }
  if (typeNode.type === "TSUnionType") {
    const values = [];
    for (const member of typeNode.types) {
      const part = literalUnion(member);
      if (!part) return null;
      values.push(...part);
    }
    return values;
  }
  return null;
}

/** The annotated type of a binding: `const x: T`, `(x: T)` or a field of a
 *  typed destructuring pattern (`({ kind }: { kind: T })`). */
function declaredType(identifier) {
  if (!identifier) return null;
  if (identifier.typeAnnotation)
    return identifier.typeAnnotation.typeAnnotation;
  const property = identifier.parent;
  const pattern = property?.parent;
  if (property?.type !== "Property" || pattern?.type !== "ObjectPattern") {
    return null;
  }
  const literal = pattern.typeAnnotation?.typeAnnotation;
  if (literal?.type !== "TSTypeLiteral") return null;
  const key = property.key.type === "Identifier" ? property.key.name : null;
  const member = literal.members.find(
    (m) =>
      m.type === "TSPropertySignature" &&
      m.key.type === "Identifier" &&
      m.key.name === key,
  );
  return member?.typeAnnotation?.typeAnnotation ?? null;
}

/** Every string an Alert `type` expression can produce, or null when the
 *  rule cannot see them all. */
function possibleTypes(node, sourceCode, depth = 0) {
  const target = unwrap(node);
  if (!target || depth > 6) return null;
  if (target.type === "Literal") {
    return typeof target.value === "string" ? [target.value] : null;
  }
  if (target.type === "TemplateLiteral" && target.expressions.length === 0) {
    return [target.quasis[0].value.cooked];
  }
  if (target.type === "ConditionalExpression") {
    const a = possibleTypes(target.consequent, sourceCode, depth + 1);
    const b = possibleTypes(target.alternate, sourceCode, depth + 1);
    return a && b ? [...a, ...b] : null;
  }
  if (target.type === "Identifier") {
    const variable = findVariable(sourceCode, target);
    if (!variable || variable.defs.length !== 1) return null;
    const def = variable.defs[0];
    const declared = literalUnion(declaredType(variable.identifiers[0]));
    if (declared) return declared;
    if (
      def.type === "Variable" &&
      def.parent?.kind === "const" &&
      def.node.init
    ) {
      return possibleTypes(def.node.init, sourceCode, depth + 1);
    }
    return null;
  }
  return null;
}

const noErrorAlert = {
  meta: {
    type: "problem",
    docs: {
      description:
        "An error Alert is rendered only by the display path (FormErrorAlert, LoadErrorAlert, TenantPage, Toast).",
    },
    messages: {
      errorAlert:
        'Kein eigenes <Alert type="error">: ein Formular- oder Speicherfehler geht über useApiFormError und FormErrorAlert, ein Ladefehler über useApiLoadError und LoadErrorAlert / TenantPage error, ein Fehler einer ganzen Aktion über useApiErrorDisplay (ADR 0006 Punkt 4). Ein Hinweis, der kein Fehlschlag ist, nimmt "warning" oder "info".',
      unprovableType:
        'Der type dieses <Alert> ist nicht als Textliteral erkennbar und kann "error" sein. Schreib die möglichen Werte aus ("warning" / "info" …); ein Fehler geht über FormErrorAlert, LoadErrorAlert oder useApiErrorDisplay (ADR 0006 Punkt 4).',
    },
    schema: [],
  },
  create(context) {
    if (!inScope(context) || PATH_MODULES_RE.test(fileName(context))) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    return {
      JSXOpeningElement(node) {
        if (!isAlertElement(node.name, sourceCode)) return;
        const typeAttribute = node.attributes.find(
          (attribute) =>
            attribute.type === "JSXAttribute" &&
            attribute.name.type === "JSXIdentifier" &&
            attribute.name.name === "type",
        );
        if (!typeAttribute) {
          if (node.attributes.some((a) => a.type === "JSXSpreadAttribute")) {
            context.report({ node, messageId: "unprovableType" });
          }
          return;
        }
        const value = typeAttribute.value;
        const expression =
          value?.type === "JSXExpressionContainer" ? value.expression : value;
        const types = possibleTypes(expression, sourceCode);
        if (types === null) {
          context.report({ node: typeAttribute, messageId: "unprovableType" });
        } else if (types.includes("error")) {
          context.report({ node: typeAttribute, messageId: "errorAlert" });
        }
      },
    };
  },
};

// --- error-path/no-error-text-display ---------------------------------------

const noErrorTextDisplay = {
  meta: {
    type: "problem",
    docs: {
      description:
        "An error's own text is never shown; the display path takes the catalog text for its code.",
    },
    messages: {
      display:
        "Der Text eines Fehlerobjekts ({{source}}) wird hier angezeigt. Angezeigt wird der Katalogtext zum Code: gib den Fehler selbst an useApiErrorDisplay / useApiFormError / useApiLoadError (ADR 0006). Für die Diagnose gehört der Text in den Logger.",
    },
    schema: [],
  },
  create(context) {
    if (!inScope(context)) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode();
    const reported = new Set();

    const report = (node) => {
      const key = node.range?.[0] ?? node.start;
      if (reported.has(key)) return;
      reported.add(key);
      context.report({
        node,
        messageId: "display",
        data: { source: sourceCode.getText(node) },
      });
    };
    const trace = (node) => traceDisplay(node, sourceCode, () => report(node));

    return {
      MemberExpression(node) {
        if (propertyName(node) !== "message") return;
        if (!isErrorObject(node.object, sourceCode)) return;
        trace(node);
      },
      CallExpression(node) {
        const callee = unwrap(node.callee);
        // String(err) and err.toString() are the error's text as well.
        if (
          callee?.type === "Identifier" &&
          callee.name === "String" &&
          node.arguments[0] &&
          isCaughtValue(node.arguments[0], sourceCode)
        ) {
          trace(node);
        } else if (
          callee?.type === "MemberExpression" &&
          propertyName(callee) === "toString" &&
          isCaughtValue(callee.object, sourceCode)
        ) {
          trace(node);
        }
        // The error itself handed to a call that renders text.
        if (isTextDisplayCall(node)) {
          for (const argument of node.arguments) {
            if (isCaughtValue(argument, sourceCode)) report(argument);
          }
        }
      },
      TemplateLiteral(node) {
        // `${err}` stringifies the error.
        for (const expression of node.expressions) {
          if (isCaughtValue(expression, sourceCode)) trace(expression);
        }
      },
      JSXExpressionContainer(node) {
        if (!isCaughtValue(node.expression, sourceCode)) return;
        const parent = node.parent;
        const rendered =
          parent?.type === "JSXElement" ||
          parent?.type === "JSXFragment" ||
          (parent?.type === "JSXAttribute" &&
            parent.name.type === "JSXIdentifier" &&
            TEXT_ATTRIBUTES.has(parent.name.name));
        if (rendered) report(node.expression);
      },
    };
  },
};

// --- error-path/no-hand-rolled-alert ----------------------------------------

// The kit renders every error people are told about: the path components
// and, at the field, the kit controls with an `error` prop (Input,
// Textarea, DatePicker). A `role="alert"` written by hand anywhere else is a
// second display path.
const KIT_DIR_RE = /(?:^|\/)src\/components\/ui\//;

function roleLiterals(node, values = []) {
  const target = unwrap(node);
  if (!target) return values;
  if (target.type === "Literal" && typeof target.value === "string") {
    values.push(target.value);
  } else if (target.type === "TemplateLiteral") {
    for (const quasi of target.quasis) values.push(quasi.value.cooked);
  } else if (target.type === "ConditionalExpression") {
    roleLiterals(target.consequent, values);
    roleLiterals(target.alternate, values);
  } else if (target.type === "LogicalExpression") {
    roleLiterals(target.left, values);
    roleLiterals(target.right, values);
  }
  return values;
}

const noHandRolledAlert = {
  meta: {
    type: "problem",
    docs: {
      description:
        'A role="alert" outside the UI kit is error display outside the path.',
    },
    messages: {
      handRolled:
        'Kein eigenes role="alert": ein Feldfehler steht über das `error`-Prop des Kit-Felds (Input, Textarea, DatePicker) am Feld, ein Formularfehler in FormErrorAlert, ein Ladefehler in LoadErrorAlert, ein Hinweis in <Alert type="warning" | "info"> (ADR 0006 Punkt 4).',
    },
    schema: [],
  },
  create(context) {
    if (!inScope(context) || KIT_DIR_RE.test(fileName(context))) return {};

    return {
      JSXAttribute(node) {
        if (node.name.type !== "JSXIdentifier" || node.name.name !== "role") {
          return;
        }
        const element = node.parent;
        if (
          element?.type !== "JSXOpeningElement" ||
          element.name.type !== "JSXIdentifier" ||
          !/^[a-z]/.test(element.name.name)
        ) {
          return;
        }
        const value =
          node.value?.type === "JSXExpressionContainer"
            ? node.value.expression
            : node.value;
        if (roleLiterals(value).some((role) => role.trim() === "alert")) {
          context.report({ node, messageId: "handRolled" });
        }
      },
    };
  },
};

// --- error-path/no-raw-error-toast ------------------------------------------

// The module that owns the toast queue and the error toast path.
const TOAST_PATH_MODULE_RE = /(?:^|\/)src\/contexts\/ToastContext\.tsx$/;

function isUseToastCall(node) {
  const target = unwrap(node);
  return (
    target?.type === "CallExpression" &&
    calleeParts(target.callee).name === "useToast" &&
    calleeParts(target.callee).object === null
  );
}

/** `toast` in `toast.error(…)`: bound to `useToast()`, or named like it. */
function isToastApi(node, sourceCode) {
  const target = unwrap(node);
  if (isUseToastCall(target)) return true;
  if (target?.type !== "Identifier") return false;
  if (/^toast(?:Api)?$/i.test(target.name)) return true;
  const variable = findVariable(sourceCode, target);
  return Boolean(
    variable?.defs.some(
      (def) =>
        def.type === "Variable" &&
        def.node.type === "VariableDeclarator" &&
        def.node.id.type === "Identifier" &&
        isUseToastCall(def.node.init),
    ),
  );
}

const noRawErrorToast = {
  meta: {
    type: "problem",
    docs: {
      description:
        "An error toast comes from useApiErrorDisplay: show(error) for a failed API call, showMessage(text) for a failure the screen found itself.",
    },
    messages: {
      rawErrorToast:
        "Kein toast.error von Hand: ein fehlgeschlagener API-Aufruf geht über useApiErrorDisplay().show(error, { object }), ein Fehler, den die Seite selbst feststellt (Kopieren, Dateigröße), über useApiErrorDisplay().showMessage(text) (ADR 0006 Punkt 4). Ein Hinweis bleibt toast.warning.",
    },
    schema: [],
  },
  create(context) {
    if (!inScope(context) || TOAST_PATH_MODULE_RE.test(fileName(context))) {
      return {};
    }
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    return {
      CallExpression(node) {
        const callee = unwrap(node.callee);
        if (
          callee?.type === "MemberExpression" &&
          propertyName(callee) === "error" &&
          isToastApi(callee.object, sourceCode)
        ) {
          context.report({ node, messageId: "rawErrorToast" });
        }
      },
      VariableDeclarator(node) {
        // const { error: toastError } = useToast();
        if (node.id.type !== "ObjectPattern" || !isUseToastCall(node.init)) {
          return;
        }
        for (const property of node.id.properties) {
          if (
            property.type === "Property" &&
            !property.computed &&
            ((property.key.type === "Identifier" &&
              property.key.name === "error") ||
              (property.key.type === "Literal" &&
                property.key.value === "error"))
          ) {
            context.report({ node: property, messageId: "rawErrorToast" });
          }
        }
      },
    };
  },
};

// --- error-path/no-error-text-match -----------------------------------------

function isRegExpReceiver(node) {
  const target = unwrap(node);
  if (!target) return false;
  if (target.type === "Literal" && target.regex) return true;
  if (target.type === "NewExpression" && target.callee.type === "Identifier") {
    return target.callee.name === "RegExp";
  }
  if (target.type === "Identifier")
    return /(?:re|regex|pattern)$/i.test(target.name);
  return false;
}

const noErrorTextMatch = {
  meta: {
    type: "problem",
    docs: {
      description:
        "No decision on an error's text: branch on ApiError.code or the HTTP status.",
    },
    messages: {
      match:
        "Entscheidung über den Fehlertext ({{source}}). Die Identität eines Fehlers ist sein Code: prüf ApiError.code oder den HTTP-Status (ADR 0006).",
    },
    schema: [],
  },
  create(context) {
    if (!inScope(context)) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    return {
      CallExpression(node) {
        const callee = unwrap(node.callee);
        if (callee?.type !== "MemberExpression") return;
        const method = propertyName(callee);
        if (
          MATCH_METHODS.has(method) &&
          isErrorText(callee.object, sourceCode)
        ) {
          context.report({
            node,
            messageId: "match",
            data: { source: sourceCode.getText(callee.object) },
          });
          return;
        }
        if (
          (method === "test" || method === "exec") &&
          isRegExpReceiver(callee.object) &&
          node.arguments[0] &&
          isErrorText(node.arguments[0], sourceCode)
        ) {
          context.report({
            node,
            messageId: "match",
            data: { source: sourceCode.getText(node.arguments[0]) },
          });
        }
      },
      BinaryExpression(node) {
        if (!["===", "!==", "==", "!="].includes(node.operator)) return;
        for (const [side, other] of [
          [node.left, node.right],
          [node.right, node.left],
        ]) {
          const literal = unwrap(other);
          const comparesText =
            (literal?.type === "Literal" &&
              typeof literal.value === "string") ||
            literal?.type === "TemplateLiteral";
          if (comparesText && isErrorText(side, sourceCode)) {
            context.report({
              node,
              messageId: "match",
              data: { source: sourceCode.getText(side) },
            });
            return;
          }
        }
      },
    };
  },
};

export default {
  meta: { name: "error-path" },
  rules: {
    "no-error-alert": noErrorAlert,
    "no-error-text-display": noErrorTextDisplay,
    "no-hand-rolled-alert": noHandRolledAlert,
    "no-raw-error-toast": noRawErrorToast,
    "no-error-text-match": noErrorTextMatch,
  },
};
