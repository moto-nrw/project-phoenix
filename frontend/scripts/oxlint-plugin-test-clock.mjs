// Deterministic test clock ratchet (issue #3101).
//
// `src/test/setup-common.ts` freezes `Date` before every test and restores
// the real clock afterwards. The freeze reaches every `new Date()` in the
// rendered component tree and in libraries, so a test cannot drift onto the
// CI host's date by accident. What the setup cannot prevent is a test that
// hands itself the real clock again; this rule catches those escapes:
//
//   test-clock/no-real-clock
//     - `vi.useRealTimers()` outside `afterEach` / `afterAll`. Inside a test
//       body or a `before*` hook it drops the frozen clock for the rest of
//       the test. Use `releaseFakeTimers()` from `~/test/clock` to give the
//       timers back to the event loop while the clock stays frozen, or
//       `useRealClock("reason")` for a reviewed real-time exception.
//     - `useRealClock()` without a non-empty string-literal reason.
//     - `vi.getRealSystemTime()`, `vi.stubGlobal("Date", …)` and assignments
//       to `globalThis.Date` / `global.Date` / `window.Date` (including
//       computed `["Date"]` properties), which read or replace the real clock
//       behind the setup's back.
//
// There is no baseline and no allowlist: an exception is `useRealClock` with
// its reason in the call, not a disable comment.

const AFTER_HOOKS = new Set(["afterEach", "afterAll"]);
const DATE_GLOBAL_OBJECTS = new Set(["globalThis", "global", "window", "self"]);
const CLOCK_HELPER_FILE = "src/test/clock.ts";

/** Repo-relative posix path ("src/…" or "scripts/…"), or "" when unavailable. */
function fileKey(context) {
  const raw = String(context.physicalFilename ?? context.filename ?? "");
  const posix = raw.replaceAll("\\", "/");
  for (const marker of ["/src/", "/scripts/"]) {
    const idx = posix.lastIndexOf(marker);
    if (idx !== -1) return posix.slice(idx + 1);
  }
  return posix;
}

function isViMember(callee, name) {
  return (
    callee.type === "MemberExpression" &&
    !callee.computed &&
    callee.object.type === "Identifier" &&
    callee.object.name === "vi" &&
    callee.property.type === "Identifier" &&
    callee.property.name === name
  );
}

function isInsideAfterHook(sourceCode, node) {
  for (const ancestor of sourceCode.getAncestors(node)) {
    if (
      ancestor.type === "CallExpression" &&
      ancestor.callee.type === "Identifier" &&
      AFTER_HOOKS.has(ancestor.callee.name)
    ) {
      return true;
    }
  }
  return false;
}

function isStringLiteral(node) {
  if (!node) return false;
  if (node.type === "Literal") return typeof node.value === "string";
  return node.type === "TemplateLiteral" && node.expressions.length === 0;
}

function literalText(node) {
  if (node.type === "Literal") return node.value;
  return node.quasis.map((quasi) => quasi.value.cooked ?? "").join("");
}

function isDateProperty(node) {
  return (
    (node.type === "Identifier" && node.name === "Date") ||
    (isStringLiteral(node) && literalText(node) === "Date")
  );
}

function isDateGlobalMember(node) {
  return (
    node.type === "MemberExpression" &&
    node.object.type === "Identifier" &&
    DATE_GLOBAL_OBJECTS.has(node.object.name) &&
    isDateProperty(node.property)
  );
}

const noRealClock = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Keep frontend tests on the deterministic test clock installed by src/test/setup-common.ts.",
    },
    messages: {
      realTimersInTest:
        'vi.useRealTimers() outside afterEach/afterAll drops the frozen test clock. Use releaseFakeTimers() from ~/test/clock to hand the timers back while the clock stays frozen, or useRealClock("reason") for a reviewed real-time exception.',
      missingReason:
        'useRealClock() needs its reviewed reason as a non-empty string literal, e.g. useRealClock("measures real elapsed time").',
      realSystemTime:
        'vi.getRealSystemTime() reads the host clock. Assert against the frozen test clock (TEST_CLOCK_INSTANT / setTestClock) or declare useRealClock("reason").',
      dateReplaced:
        "Replacing the global Date bypasses the deterministic test clock. Use setTestClock() / vi.setSystemTime() to move time, or vi.useFakeTimers() to control it.",
    },
    schema: [],
  },
  create(context) {
    if (fileKey(context) === CLOCK_HELPER_FILE) return {};
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    return {
      CallExpression(node) {
        const { callee } = node;

        if (isViMember(callee, "useRealTimers")) {
          if (!isInsideAfterHook(sourceCode, node)) {
            context.report({ node, messageId: "realTimersInTest" });
          }
          return;
        }

        if (isViMember(callee, "getRealSystemTime")) {
          context.report({ node, messageId: "realSystemTime" });
          return;
        }

        if (
          isViMember(callee, "stubGlobal") &&
          isStringLiteral(node.arguments[0]) &&
          literalText(node.arguments[0]) === "Date"
        ) {
          context.report({ node, messageId: "dateReplaced" });
          return;
        }

        if (callee.type === "Identifier" && callee.name === "useRealClock") {
          const reason = node.arguments[0];
          if (!isStringLiteral(reason) || literalText(reason).trim() === "") {
            context.report({ node, messageId: "missingReason" });
          }
        }
      },
      AssignmentExpression(node) {
        if (isDateGlobalMember(node.left)) {
          context.report({ node, messageId: "dateReplaced" });
        }
      },
    };
  },
};

export default {
  meta: { name: "test-clock" },
  rules: { "no-real-clock": noRealClock },
};
