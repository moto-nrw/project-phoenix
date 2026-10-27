// #5101: class names are free text. Some schools store "1b", others
// "Klasse 1b"; a hand-written "Klasse " in front of the stored name reads
// "Klasse Klasse 1b". src/lib/school-class-label.ts is the only place that
// adds the word. This rule rejects a "Klasse " / "Klassen " that a template
// literal, a JSX text or a string concatenation puts directly in front of a
// value. A grade level ("Klasse 3", Jahrgangsstufe) is a number, not a
// class name: a value whose source names a grade (grade, gradeLevel,
// grade_level, jahrgang, stufe) stays allowed.

const HELPER_FILE = "src/lib/school-class-label.ts";
const EXEMPT_FILE = /\.(test|spec|stories)\.[cm]?[jt]sx?$/;
// "Klasse " or "Klassen " at the end of a text, not "Unterklasse ".
const ENDS_WITH_PREFIX = /(?:^|[^\p{L}])Klassen?\s+$/iu;
const ENDS_WITH_WORD = /(?:^|[^\p{L}])Klassen?$/iu;
const GRADE_VALUE = /grade|jahrgang|stufe/i;

function fileKey(context) {
  const file = String(
    context.physicalFilename ?? context.filename ?? "",
  ).replaceAll("\\", "/");
  const index = file.lastIndexOf("/src/");
  return index === -1 ? file : file.slice(index + 1);
}

function stringValue(node) {
  if (node?.type === "Literal" && typeof node.value === "string") {
    return node.value;
  }
  return null;
}

// The text a "+" chain ends with: "Klasse " + x, or a + "Klasse " + x.
function trailingText(node) {
  const literal = stringValue(node);
  if (literal !== null) return literal;
  if (node?.type === "TemplateLiteral") {
    return node.quasis.at(-1)?.value.cooked ?? null;
  }
  if (node?.type === "BinaryExpression" && node.operator === "+") {
    return trailingText(node.right);
  }
  return null;
}

const noHandBuiltLabel = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Label school classes through ~/lib/school-class-label, never by writing 'Klasse ' in front of a class name.",
    },
    schema: [],
    messages: {
      handBuilt:
        "Do not write 'Klasse ' in front of a class name: schools that store 'Klasse 1b' then read 'Klasse Klasse 1b' (#5101). Use schoolClassLabel, schoolClassesLabel or schoolClassName from ~/lib/school-class-label. A grade level may keep the word when its value is named after the grade (grade, gradeLevel, jahrgang).",
    },
  },
  create(context) {
    const file = fileKey(context);
    if (file === HELPER_FILE || EXEMPT_FILE.test(file)) return {};
    const sourceCode = context.sourceCode;

    function isGradeValue(node) {
      return GRADE_VALUE.test(sourceCode.getText(node));
    }

    function check(valueNode, reportNode) {
      if (!valueNode || valueNode.type === "JSXEmptyExpression") return;
      if (valueNode.type === "Literal") return;
      if (isGradeValue(valueNode)) return;
      context.report({ node: reportNode, messageId: "handBuilt" });
    }

    function checkChildren(children) {
      for (let i = 0; i < children.length; i += 1) {
        const child = children[i];
        if (child.type !== "JSXText") continue;
        let next = i + 1;
        if (!ENDS_WITH_PREFIX.test(child.value)) {
          // Klasse{" "}{name}
          const space = children[next];
          if (
            !ENDS_WITH_WORD.test(child.value) ||
            space?.type !== "JSXExpressionContainer" ||
            stringValue(space.expression)?.trim() !== ""
          ) {
            continue;
          }
          next += 1;
        }
        const value = children[next];
        if (value?.type === "JSXExpressionContainer") {
          check(value.expression, value);
        }
      }
    }

    return {
      TemplateLiteral(node) {
        node.expressions.forEach((expression, index) => {
          const before = node.quasis[index]?.value.cooked ?? "";
          if (ENDS_WITH_PREFIX.test(before)) check(expression, node);
        });
      },
      BinaryExpression(node) {
        if (node.operator !== "+") return;
        const before = trailingText(node.left);
        if (before !== null && ENDS_WITH_PREFIX.test(before)) {
          check(node.right, node);
        }
      },
      JSXElement(node) {
        checkChildren(node.children);
      },
      JSXFragment(node) {
        checkChildren(node.children);
      },
    };
  },
};

export default {
  meta: { name: "school-class" },
  rules: { "no-hand-built-label": noHandBuiltLabel },
};
