// Absence assertions need a positive check (issue #4445).
//
// `queryBy…` with `not.toBeInTheDocument()`, `toBeNull()` or `toHaveLength(0)`
// also passes against an empty render, a permanent skeleton or renamed text.
// docs/agents/frontend-testing.md ("Absence assertions") describes the anchor
// pattern; these rules keep new tests from skipping it:
//
//   absence-assertion/needs-anchor
//     A test whose only DOM assertions are absences. It needs positive DOM
//     evidence in the same test: a `getBy…` / `findBy…` call (they throw when
//     the element is missing), a positive jest-dom matcher, or a positive
//     assertion on a DOM node or its text.
//
//   absence-assertion/no-early-wait-for
//     A `waitFor` callback with an absence assertion and no positive DOM
//     evidence before it. Such a `waitFor` passes on its first run, before
//     asynchronous data arrives. Wait for the loaded state first
//     (`await screen.findBy…`), then assert the absence synchronously.
//
// Evidence counts from the test body, from the `beforeEach` / `beforeAll`
// hooks of the enclosing `describe` blocks, and from functions defined in the
// same file that the test calls. Imported helpers are opaque.
//
// Exception, not baseline: a test that asserts the exact empty render
// (`expect(container.firstChild).toBeNull()`,
// `expect(container).toBeEmptyDOMElement()` or
// `expect(container.innerHTML).toBe("")`) documents a component that
// renders nothing; it has nothing positive to anchor on. There is no
// allowlist; a test that cannot anchor is rewritten, not exempted.

const TEST_NAMES = new Set(["it", "test"]);
const HOOK_NAMES = new Set(["beforeEach", "beforeAll"]);
const TEST_MODIFIERS = new Set([
  "only",
  "skip",
  "concurrent",
  "sequential",
  "fails",
  "each",
  "for",
  "todo",
  "skipIf",
  "runIf",
]);

const POSITIVE_QUERY_RE = /^(get|find)(All)?By[A-Z]/;
const ABSENT_QUERY_RE = /^query(All)?By[A-Z]/;
const SINGLE_NODE_METHODS = new Set([
  "querySelector",
  "getElementById",
  "closest",
]);
const NODE_LIST_METHODS = new Set([
  "querySelectorAll",
  "getElementsByClassName",
  "getElementsByTagName",
  "getElementsByName",
]);
const TEXT_PROPERTIES = new Set([
  "textContent",
  "innerHTML",
  "outerHTML",
  "innerText",
]);
const NODE_PROPERTIES = new Set([
  "className",
  "classList",
  "dataset",
  "style",
  "tagName",
  "children",
  "childNodes",
  "firstChild",
  "firstElementChild",
  "lastChild",
  "parentElement",
  "getAttribute",
  "hasAttribute",
]);
const ROOT_NAMES = new Set(["container", "baseElement", "document", "screen"]);

// jest-dom matchers that throw on a missing element, negated or not; only
// `toBeInTheDocument` accepts null under `.not`.
const JEST_DOM_MATCHERS = new Set([
  "toBeInTheDocument",
  "toHaveTextContent",
  "toHaveAttribute",
  "toHaveClass",
  "toBeVisible",
  "toBeChecked",
  "toBeDisabled",
  "toBeEnabled",
  "toHaveValue",
  "toHaveDisplayValue",
  "toHaveStyle",
  "toHaveFocus",
  "toHaveAccessibleName",
  "toHaveAccessibleDescription",
  "toHaveAccessibleErrorMessage",
  "toHaveErrorMessage",
  "toContainElement",
  "toContainHTML",
  "toHaveFormValues",
  "toBeRequired",
  "toBeInvalid",
  "toBeValid",
  "toBePartiallyChecked",
  "toHaveRole",
  "toHaveSelection",
  "toBeEmptyDOMElement",
]);
const NULLISH_MATCHERS = new Set(["toBeNull", "toBeFalsy"]);
const TRUTHY_MATCHERS = new Set(["toBeTruthy"]);
const TEXT_MATCHERS = new Set([
  "toContain",
  "toMatch",
  "toHaveTextContent",
  "toContainHTML",
]);
const EMPTY_LIST_MATCHERS = new Set(["toEqual", "toStrictEqual"]);

function calleeName(callee) {
  if (!callee) return "";
  if (callee.type === "Identifier") return callee.name;
  if (callee.type === "MemberExpression" && !callee.computed) {
    return callee.property.name ?? "";
  }
  return "";
}

/** Base identifier of `it`, `it.only`, `it.each(table)`, `test.skip.each(t)`. */
function testBaseName(callee) {
  let current = callee;
  for (;;) {
    if (current.type === "Identifier") return current.name;
    if (current.type === "CallExpression") {
      current = current.callee;
      continue;
    }
    if (
      current.type === "MemberExpression" &&
      !current.computed &&
      TEST_MODIFIERS.has(current.property.name)
    ) {
      current = current.object;
      continue;
    }
    return "";
  }
}

function isTestCall(node) {
  return TEST_NAMES.has(testBaseName(node.callee));
}

function isHookCall(node) {
  return node.callee.type === "Identifier" && HOOK_NAMES.has(node.callee.name);
}

function isDescribeCall(node) {
  return testBaseName(node.callee) === "describe";
}

function isWaitForCall(node) {
  const { callee } = node;
  if (callee.type === "Identifier") return callee.name === "waitFor";
  return (
    callee.type === "MemberExpression" &&
    !callee.computed &&
    callee.property.name === "waitFor"
  );
}

function isFunction(node) {
  return (
    node.type === "ArrowFunctionExpression" ||
    node.type === "FunctionExpression" ||
    node.type === "FunctionDeclaration"
  );
}

function unwrap(node) {
  let current = node;
  while (
    current &&
    (current.type === "ChainExpression" ||
      current.type === "TSNonNullExpression" ||
      current.type === "AwaitExpression" ||
      current.type === "TSAsExpression" ||
      current.type === "ParenthesizedExpression")
  ) {
    current = current.expression ?? current.argument;
  }
  return current;
}

/** `{ chain: ["not", "toBeNull"], subject }` for `expect(subject).not.toBeNull()`. */
function expectChain(node) {
  if (node.callee.type !== "MemberExpression") return null;
  const chain = [];
  let current = node.callee;
  while (current.type === "MemberExpression") {
    if (current.computed) return null;
    chain.unshift(current.property.name);
    current = current.object;
  }
  if (current.type !== "CallExpression") return null;
  const head = current.callee;
  const isExpect =
    (head.type === "Identifier" && head.name === "expect") ||
    (head.type === "MemberExpression" &&
      !head.computed &&
      head.object.type === "Identifier" &&
      head.object.name === "expect" &&
      head.property.name === "soft");
  if (!isExpect) return null;
  return { chain, subject: current.arguments[0] };
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

/**
 * What a matcher subject points at: "positive" (a getBy/findBy result),
 * "node" (a single queried node that may be null), "list" (a node list),
 * "text" (text or markup of a node), "root" (container, document),
 * "dom" (another DOM property) or "" for everything else.
 */
function subjectKind(sourceCode, node, depth = 0) {
  const subject = unwrap(node);
  if (!subject || depth > 3) return "";
  if (subject.type === "CallExpression") {
    const name = calleeName(subject.callee);
    if (POSITIVE_QUERY_RE.test(name)) return "positive";
    if (ABSENT_QUERY_RE.test(name)) {
      return name.startsWith("queryAll") ? "list" : "node";
    }
    if (SINGLE_NODE_METHODS.has(name)) return "node";
    if (NODE_LIST_METHODS.has(name)) return "list";
    if (name === "getAttribute" || name === "hasAttribute") return "dom";
    return "";
  }
  if (subject.type === "MemberExpression") {
    const property = subject.computed ? "" : subject.property.name;
    if (TEXT_PROPERTIES.has(property)) return "text";
    if (property === "firstChild" && isContainer(subject.object)) {
      return "node";
    }
    if (property === "body" && isIdentifier(subject.object, "document")) {
      return "root";
    }
    if (NODE_PROPERTIES.has(property)) return "dom";
    if (property === "length") {
      const inner = subjectKind(sourceCode, subject.object, depth + 1);
      return inner === "list" ? "length" : "";
    }
    return "";
  }
  if (subject.type === "Identifier") {
    if (ROOT_NAMES.has(subject.name)) return "root";
    const variable = findVariable(sourceCode, subject);
    const init = variable?.defs[0]?.node?.init;
    if (init && variable.defs[0].node.id?.type === "Identifier") {
      return subjectKind(sourceCode, init, depth + 1);
    }
  }
  return "";
}

function isIdentifier(node, name) {
  return node?.type === "Identifier" && node.name === name;
}

function isContainer(node) {
  const target = unwrap(node);
  return (
    isIdentifier(target, "container") ||
    (target?.type === "MemberExpression" &&
      !target.computed &&
      target.property.name === "container")
  );
}

function isZeroLiteral(node) {
  return node?.type === "Literal" && node.value === 0;
}

function isEmptyString(node) {
  return (
    (node?.type === "Literal" && node.value === "") ||
    (node?.type === "TemplateLiteral" &&
      node.expressions.length === 0 &&
      node.quasis.every((quasi) => (quasi.value.cooked ?? "") === ""))
  );
}

function isEmptyArray(node) {
  return node?.type === "ArrayExpression" && node.elements.length === 0;
}

/** "absent", "positive", "empty-render" or "" for one expect chain. */
function classifyAssertion(sourceCode, call, { chain, subject }) {
  const matcher = chain.at(-1);
  const negated = chain.includes("not");
  const argument = call.arguments[0];
  const target = unwrap(subject);

  if (
    !negated &&
    ((matcher === "toBeNull" &&
      target?.type === "MemberExpression" &&
      !target.computed &&
      target.property.name === "firstChild" &&
      isContainer(target.object)) ||
      (matcher === "toBeEmptyDOMElement" && isContainer(target)) ||
      // `innerHTML` is empty only without any element; `textContent` is
      // also empty for a skeleton of bare boxes and stays an absence.
      (matcher === "toBe" &&
        isEmptyString(argument) &&
        target?.type === "MemberExpression" &&
        !target.computed &&
        target.property.name === "innerHTML" &&
        isContainer(target.object)))
  ) {
    return "empty-render";
  }

  if (matcher === "toBeInTheDocument") return negated ? "absent" : "positive";
  // Outside the empty-render contract, emptiness proves nothing either way:
  // `not.toBeEmptyDOMElement()` also holds for a skeleton.
  if (matcher === "toBeEmptyDOMElement") return "";
  if (JEST_DOM_MATCHERS.has(matcher)) {
    // `expect(container).not.toHaveTextContent("x")` holds for an empty render.
    if (negated && TEXT_MATCHERS.has(matcher)) {
      const kind = subjectKind(sourceCode, subject);
      if (kind === "root" || kind === "text") return "absent";
    }
    return "positive";
  }

  const kind = subjectKind(sourceCode, subject);
  if (kind === "") return "";
  if (kind === "positive") return "positive";

  if (kind === "node") {
    // Nullable queries return null, not undefined. Neither check proves presence.
    if (matcher === "toBeDefined" || matcher === "toBeUndefined") return "";
    if (
      (matcher === "toBe" || EMPTY_LIST_MATCHERS.has(matcher)) &&
      argument?.type === "Literal" &&
      argument.value === null
    ) {
      return negated ? "positive" : "absent";
    }
    if (NULLISH_MATCHERS.has(matcher)) return negated ? "positive" : "absent";
    if (TRUTHY_MATCHERS.has(matcher)) return negated ? "absent" : "positive";
    return "positive";
  }
  if (kind === "list" || kind === "length") {
    const lengthMatcher = kind === "list" ? "toHaveLength" : "toBe";
    if (
      matcher === lengthMatcher ||
      (kind === "length" && EMPTY_LIST_MATCHERS.has(matcher))
    ) {
      if (isZeroLiteral(argument)) return negated ? "positive" : "absent";
      // Only a known positive count rules out an empty result.
      return !negated &&
        argument?.type === "Literal" &&
        Number.isInteger(argument.value) &&
        argument.value > 0
        ? "positive"
        : "";
    }
    if (
      kind === "list" &&
      EMPTY_LIST_MATCHERS.has(matcher) &&
      isEmptyArray(argument)
    ) {
      return negated ? "positive" : "absent";
    }
    // Arrays and NodeLists are truthy and non-null even when empty.
    return kind === "list" ? "" : "positive";
  }
  if (kind === "text" || kind === "root") {
    if (negated && (TEXT_MATCHERS.has(matcher) || matcher === "toBe")) {
      return "absent";
    }
    // `expect(container.textContent).toBe("")` holds for an empty render.
    if (
      !negated &&
      (((matcher === "toBe" || EMPTY_LIST_MATCHERS.has(matcher)) &&
        isEmptyString(argument)) ||
        (matcher === "toHaveLength" && isZeroLiteral(argument)))
    ) {
      return "absent";
    }
    return "positive";
  }
  return negated ? "" : "positive";
}

/** Name under which a function can be called from a test, or "". */
function helperName(fn) {
  if (fn.type === "FunctionDeclaration") return fn.id?.name ?? "";
  const parent = fn.parent;
  if (
    parent?.type === "VariableDeclarator" &&
    parent.id.type === "Identifier"
  ) {
    return parent.id.name;
  }
  return "";
}

function testTitle(sourceCode, call) {
  const title = call.arguments[0];
  if (title?.type === "Literal" && typeof title.value === "string") {
    return title.value;
  }
  if (title?.type === "TemplateLiteral" && title.expressions.length === 0) {
    return title.quasis.map((quasi) => quasi.value.cooked ?? "").join("");
  }
  return title ? sourceCode.getText(title) : "";
}

function within(outer, inner) {
  return outer.range[0] <= inner.range[0] && inner.range[1] <= outer.range[1];
}

/**
 * Collects positive and absence evidence per owner (test callback, hook or
 * named helper) in one pass and evaluates both rules at `Program:exit`.
 * Both rules share the collector; each reports only its own finding.
 */
function createAnalysis(context, report) {
  const sourceCode = context.sourceCode ?? context.getSourceCode();
  const owners = new Map(); // owner node -> { kind, node, events, … }
  const describeFns = [];

  function ownerOf(node) {
    let current = node.parent;
    let child = node;
    let waitFor = null;
    while (current) {
      if (
        !waitFor &&
        current.type === "CallExpression" &&
        isWaitForCall(current) &&
        current.arguments[0] === child
      ) {
        waitFor = current;
      }
      if (isFunction(current)) {
        const owner = ensureOwner(current);
        if (owner) return { owner, waitFor };
      }
      child = current;
      current = current.parent;
    }
    return null;
  }

  function ensureOwner(fn) {
    if (owners.has(fn)) return owners.get(fn);
    const parent = fn.parent;
    let owner = null;
    if (parent?.type === "CallExpression" && parent.arguments.includes(fn)) {
      if (isTestCall(parent)) owner = { kind: "test", call: parent };
      else if (isHookCall(parent)) owner = { kind: "hook", call: parent };
    }
    if (!owner) {
      const name = helperName(fn);
      if (name) {
        owner = { kind: "helper", name };
      }
    }
    if (!owner) return null;
    owner.fn = fn;
    owner.events = [];
    owners.set(fn, owner);
    return owner;
  }

  function record(node, event) {
    const located = ownerOf(node);
    if (!located) return;
    located.owner.events.push({
      ...event,
      node,
      waitFor: located.waitFor,
      start: node.range[0],
    });
  }

  function resolveHelper(callNode) {
    const variable = findVariable(sourceCode, callNode.callee);
    const definition = variable?.defs[0]?.node;
    const fn =
      definition?.type === "FunctionDeclaration"
        ? definition
        : unwrap(definition?.init);
    return fn && isFunction(fn) ? ensureOwner(fn) : null;
  }

  function flatten(owner, visited, out) {
    if (visited.has(owner)) return;
    visited.add(owner);
    const events = owner.events.toSorted((a, b) => a.start - b.start);
    for (const event of events) {
      if (event.kind === "call") {
        const helper = resolveHelper(event.node);
        if (helper) {
          const nested = [];
          flatten(helper, visited, nested);
          for (const inner of nested) {
            out.push({ ...inner, waitFor: inner.waitFor ?? event.waitFor });
          }
        }
        continue;
      }
      out.push(event);
    }
    visited.delete(owner);
  }

  function hooksFor(testOwner) {
    const hooks = [];
    for (const owner of owners.values()) {
      if (owner.kind !== "hook") continue;
      const scope = describeFns
        .filter((fn) => within(fn, owner.call))
        .toSorted((a, b) => b.range[0] - a.range[0])[0];
      if (!scope || within(scope, testOwner.call)) hooks.push(owner);
    }
    return hooks.toSorted((a, b) => a.call.range[0] - b.call.range[0]);
  }

  return {
    CallExpression(node) {
      if (isDescribeCall(node)) {
        const fn = node.arguments.find((argument) => isFunction(argument));
        if (fn) describeFns.push(fn);
      }
      for (const argument of node.arguments) {
        if (isFunction(argument)) ensureOwner(argument);
      }

      const chain = expectChain(node);
      if (chain) {
        const verdict = classifyAssertion(sourceCode, node, chain);
        if (verdict) record(node, { kind: verdict });
        return;
      }

      const name = calleeName(node.callee);
      if (
        POSITIVE_QUERY_RE.test(name) ||
        name === "waitForElementToBeRemoved"
      ) {
        record(node, { kind: "positive" });
        return;
      }
      if (node.callee.type === "Identifier") {
        // A same-file helper may be declared later; resolved at Program:exit.
        record(node, { kind: "call" });
      }
    },
    FunctionDeclaration(node) {
      ensureOwner(node);
    },
    "Program:exit"() {
      for (const owner of owners.values()) {
        if (owner.kind !== "test") continue;
        const events = [];
        for (const hook of hooksFor(owner)) flatten(hook, new Set(), events);
        flatten(owner, new Set(), events);

        if (events.some((event) => event.kind === "empty-render")) continue;

        let positive = false;
        let absent = false;
        // A retry only succeeds when every assertion in its callback passes,
        // regardless of their order. Keep this evidence local to that waitFor.
        const anchoredWaitFors = new Set(
          events
            .filter((event) => event.kind === "positive" && event.waitFor)
            .map((event) => event.waitFor),
        );
        const earlyWaitFors = new Set();
        for (const event of events) {
          if (event.kind === "positive") positive = true;
          if (event.kind !== "absent") continue;
          absent = true;
          if (
            event.waitFor &&
            !positive &&
            !anchoredWaitFors.has(event.waitFor)
          ) {
            earlyWaitFors.add(event.waitFor);
          }
        }

        report({
          owner,
          title: testTitle(sourceCode, owner.call),
          absenceOnly: absent && !positive,
          earlyWaitFors: [...earlyWaitFors],
        });
      }
    },
  };
}

const needsAnchor = {
  meta: {
    type: "problem",
    docs: {
      description:
        "A test whose only DOM assertions are absences also passes against an empty render or a permanent skeleton.",
    },
    messages: {
      absenceOnly:
        "Test „{{title}}“ prüft im DOM nur Abwesenheit. Das besteht auch bei leerem Render oder dauerhaftem Ladezustand. Prüfe vorher ein Element, das nur der geladene Inhalt zeigt (`// Anker: …`, docs/agents/frontend-testing.md „Absence assertions“). Ausnahme: der exakte Leer-Render-Vertrag `expect(container.firstChild).toBeNull()`.",
    },
    schema: [],
  },
  create(context) {
    return createAnalysis(context, ({ owner, title, absenceOnly }) => {
      if (!absenceOnly) return;
      context.report({
        node: owner.call,
        messageId: "absenceOnly",
        data: { title },
      });
    });
  },
};

const noEarlyWaitFor = {
  meta: {
    type: "problem",
    docs: {
      description:
        "A waitFor with an absence assertion passes on its first run unless a positive state was awaited before it.",
    },
    messages: {
      earlyWaitFor:
        "Dieses `waitFor` prüft eine Abwesenheit, bevor der Test einen positiven Zustand gesehen hat, und besteht deshalb schon im ersten Durchlauf. Warte zuerst auf den geladenen Inhalt (`await screen.findBy…`) und prüfe die Abwesenheit danach synchron (docs/agents/frontend-testing.md „Absence assertions“).",
    },
    schema: [],
  },
  create(context) {
    // A waitFor inside a shared helper is reported once, not once per test.
    const reported = new Set();
    return createAnalysis(context, ({ earlyWaitFors }) => {
      for (const node of earlyWaitFors) {
        if (reported.has(node)) continue;
        reported.add(node);
        context.report({ node, messageId: "earlyWaitFor" });
      }
    });
  },
};

export default {
  meta: { name: "absence-assertion" },
  rules: {
    "needs-anchor": needsAnchor,
    "no-early-wait-for": noEarlyWaitFor,
  },
};
