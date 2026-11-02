import { readdirSync, statSync } from "node:fs";
import path from "node:path";
import ts from "typescript";

/**
 * The cookie writers of the session cookie inventory (#5399, #5444):
 * session-cookie-inventory.test.ts lists every file that can write a cookie
 * and keeps the session cookies on gateSessionCookieWrites.
 *
 * A tripwire for known patterns, not a guarantee: static analysis of
 * JavaScript cannot be complete. It reads the syntax tree, not the text.
 * Every use of a cookie jar or cookie list that is not a known read counts as
 * a write, including aliases, destructuring, string keys and values handed to
 * other code. A computed key resolves through `const` names; one that does
 * not resolve counts on a request, a response or headers, and right before
 * `.set`, `.delete` or `.append` on anything. Exports that carry a Set-Cookie
 * or session cookie name, also through a later Object.assign, property
 * assignment or spread, are listed.
 *
 * Known limits, among others: a key or header name computed at run time
 * (other than from literals and constants) on an unrecognised receiver, a
 * write through a helper in another module whose own file passes, and a
 * backend response's headers forwarded as a whole (#5517). Runtime
 * enforcement at the exit is #5520.
 */

export type CookieWriteKind =
  /** Next's cookie jar of the request: cookies() of next/headers. */
  | "next-cookie-jar"
  /** A cookie list: response.cookies, request.cookies, ResponseCookies, cookieStore. */
  | "cookie-list"
  /** A Set-Cookie header name. */
  | "cookie-header"
  /** document.cookie, or document handed to code that can set it. */
  | "document-cookie"
  /**
   * A member of a request, a response or headers under a key that is no
   * constant: it may be their cookie list.
   */
  | "computed-member"
  /** A cookie library from outside this repository. */
  | "cookie-library"
  /** A cookie writer leaving its module, out of reach of a per-file check. */
  | "module-export";

export interface CookieWrite {
  kind: CookieWriteKind;
  line: number;
  code: string;
}

/** What a tracked expression holds. */
type Value =
  | "headers-module"
  | "jar-factory"
  | "jar"
  | "list-factory"
  | "list"
  | "global"
  | "document"
  | "cookie-header-name";

const KIND: Record<Value, CookieWriteKind> = {
  "headers-module": "next-cookie-jar",
  "jar-factory": "next-cookie-jar",
  jar: "next-cookie-jar",
  "list-factory": "cookie-list",
  list: "cookie-list",
  global: "document-cookie",
  document: "document-cookie",
  "cookie-header-name": "cookie-header",
};

/** Members that only read a cookie list or Next's cookie jar. */
const READS = new Set(["get", "getAll", "has", "size", "toString"]);
/** Auth.js' cookie options of a portal config: `config.cookies.sessionToken`. */
const LIST_READS = new Set([...READS, "sessionToken"]);
const LIST_FACTORIES = new Set(["ResponseCookies", "RequestCookies"]);
/** The browser's global objects, which hold document and cookieStore. */
const GLOBALS = new Set([
  "window",
  "globalThis",
  "self",
  "parent",
  "top",
  "frames",
]);
/** Constructors and factories of requests, responses and headers. */
const MESSAGE_CLASSES = new Set([
  "Request",
  "NextRequest",
  "Response",
  "NextResponse",
  "Headers",
]);
/** Types of requests, responses and headers, as annotated. */
const MESSAGE_TYPES = new Set([...MESSAGE_CLASSES, "ReadonlyHeaders"]);
/** Names that conventionally hold a request or a response. */
const MESSAGE_NAMES = /^(?:req|request|res|response)$/i;
/** Members that hold a document or a window on any object. */
const DOCUMENT_MEMBERS = new Set(["contentDocument", "ownerDocument"]);

// The class keeps this pattern from spelling the header name it looks for.
const SET_COOKIE = /set[-]cookie/i;
const SESSION_TOKEN = /session-token/;

function parse(file: string, source: string): ts.SourceFile {
  const kind = file.endsWith(".tsx")
    ? ts.ScriptKind.TSX
    : file.endsWith(".jsx")
      ? ts.ScriptKind.JSX
      : /\.[cm]?js$/.test(file)
        ? ts.ScriptKind.JS
        : ts.ScriptKind.TS;
  return ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, kind);
}

function walk(node: ts.Node, visit: (node: ts.Node) => void): void {
  visit(node);
  ts.forEachChild(node, (child) => walk(child, visit));
}

/**
 * The value of a string expression built from literals and `const` names
 * bound to such expressions in the same file.
 */
function stringValue(
  node: ts.Node,
  seen: Set<ts.Node> = new Set(),
): string | undefined {
  if (ts.isStringLiteralLike(node) || ts.isNumericLiteral(node))
    return node.text;
  if (
    ts.isParenthesizedExpression(node) ||
    ts.isAsExpression(node) ||
    ts.isSatisfiesExpression(node) ||
    ts.isNonNullExpression(node)
  )
    return stringValue(node.expression, seen);
  if (ts.isIdentifier(node)) {
    const declared = declarationOf(node);
    const declaration = declared?.parent;
    if (
      !declared ||
      seen.has(declared) ||
      !declaration ||
      !ts.isVariableDeclaration(declaration) ||
      declaration.name !== declared ||
      !declaration.initializer ||
      !(declaration.parent.flags & ts.NodeFlags.Const)
    )
      return undefined;
    return stringValue(declaration.initializer, new Set([...seen, declared]));
  }
  if (
    ts.isBinaryExpression(node) &&
    node.operatorToken.kind === ts.SyntaxKind.PlusToken
  ) {
    const left = stringValue(node.left, seen);
    const right = stringValue(node.right, seen);
    return left === undefined || right === undefined ? undefined : left + right;
  }
  if (ts.isTemplateExpression(node)) {
    let text = node.head.text;
    for (const span of node.templateSpans) {
      const value = stringValue(span.expression, seen);
      if (value === undefined) return undefined;
      text += value + span.literal.text;
    }
    return text;
  }
  return undefined;
}

/** Text a node spells: a string, a template part or a folded expression. */
function spelledText(node: ts.Node): string | undefined {
  if (
    ts.isTemplateHead(node) ||
    ts.isTemplateMiddle(node) ||
    ts.isTemplateTail(node)
  )
    return node.text;
  if (
    ts.isStringLiteralLike(node) ||
    ts.isTemplateExpression(node) ||
    (ts.isBinaryExpression(node) &&
      node.operatorToken.kind === ts.SyntaxKind.PlusToken)
  )
    return stringValue(node);
  return undefined;
}

function memberName(node: ts.Node): string | undefined {
  if (ts.isPropertyAccessExpression(node)) return node.name.text;
  if (ts.isElementAccessExpression(node))
    return stringValue(node.argumentExpression);
  return undefined;
}

function propertyNameText(name: ts.PropertyName): string | undefined {
  if (ts.isComputedPropertyName(name)) return stringValue(name.expression);
  return name.text;
}

/** The property a binding element takes: `{ set }`, `{ set: put }`. */
function boundProperty(element: ts.BindingElement): string | undefined {
  if (element.propertyName) return propertyNameText(element.propertyName);
  return ts.isIdentifier(element.name) ? element.name.text : undefined;
}

function isAssignment(kind: ts.SyntaxKind): boolean {
  return (
    kind >= ts.SyntaxKind.FirstAssignment &&
    kind <= ts.SyntaxKind.LastAssignment
  );
}

/** Whether the expression is written to, also inside a destructuring target. */
function isWriteTarget(node: ts.Node): boolean {
  let current = node;
  for (;;) {
    const parent = current.parent;
    if (
      ts.isParenthesizedExpression(parent) ||
      ts.isArrayLiteralExpression(parent) ||
      ts.isObjectLiteralExpression(parent) ||
      ts.isSpreadElement(parent) ||
      ts.isSpreadAssignment(parent) ||
      (ts.isPropertyAssignment(parent) && parent.initializer === current)
    ) {
      current = parent;
      continue;
    }
    if (ts.isBinaryExpression(parent))
      return parent.left === current && isAssignment(parent.operatorToken.kind);
    if (
      ts.isPrefixUnaryExpression(parent) ||
      ts.isPostfixUnaryExpression(parent)
    )
      return (
        parent.operator === ts.SyntaxKind.PlusPlusToken ||
        parent.operator === ts.SyntaxKind.MinusMinusToken
      );
    if (ts.isForOfStatement(parent) || ts.isForInStatement(parent))
      return parent.initializer === current;
    return ts.isDeleteExpression(parent);
  }
}

function isDeleted(node: ts.Node): boolean {
  let current = node;
  while (ts.isParenthesizedExpression(current.parent)) current = current.parent;
  return ts.isDeleteExpression(current.parent);
}

/** Methods that write a cookie list or headers. */
const WRITE_METHODS = new Set(["set", "delete", "append"]);

/** `x[key].set(…)`: a computed member right before a write method. */
function isBeforeWriteMethod(node: ts.ElementAccessExpression): boolean {
  const parent = node.parent;
  return (
    (ts.isPropertyAccessExpression(parent) ||
      ts.isElementAccessExpression(parent)) &&
    parent.expression === node &&
    WRITE_METHODS.has(memberName(parent) ?? "")
  );
}

/** Whether the value only decides a condition. */
function isCondition(node: ts.Node): boolean {
  const parent = node.parent;
  if (
    (ts.isIfStatement(parent) ||
      ts.isWhileStatement(parent) ||
      ts.isDoStatement(parent)) &&
    parent.expression === node
  )
    return true;
  if (ts.isForStatement(parent)) return parent.condition === node;
  if (ts.isConditionalExpression(parent)) return parent.condition === node;
  if (ts.isPrefixUnaryExpression(parent))
    return parent.operator === ts.SyntaxKind.ExclamationToken;
  if (ts.isTypeOfExpression(parent)) return true;
  if (ts.isBinaryExpression(parent))
    return [
      ts.SyntaxKind.EqualsEqualsEqualsToken,
      ts.SyntaxKind.ExclamationEqualsEqualsToken,
      ts.SyntaxKind.EqualsEqualsToken,
      ts.SyntaxKind.ExclamationEqualsToken,
      ts.SyntaxKind.AmpersandAmpersandToken,
    ].includes(parent.operatorToken.kind);
  return false;
}

function isExternalModule(specifier: string): boolean {
  return !/^(\.|~\/|@\/|\/)/.test(specifier);
}

/** A package that reads or writes cookies, from outside this repository. */
function isCookieLibrary(specifier: string): boolean {
  return isExternalModule(specifier) && /cookie/i.test(specifier);
}

/** The value names a scope node declares itself. */
function declaredNames(node: ts.Node): ts.Identifier[] {
  const names: ts.Identifier[] = [];
  const add = (name: ts.BindingName) => {
    if (ts.isIdentifier(name)) names.push(name);
    else
      for (const element of name.elements)
        if (!ts.isOmittedExpression(element)) add(element.name);
  };
  if (ts.isFunctionLike(node)) {
    for (const parameter of node.parameters) add(parameter.name);
    if (ts.isFunctionExpression(node) && node.name) names.push(node.name);
  }
  if (ts.isClassExpression(node) && node.name) names.push(node.name);
  if (ts.isCatchClause(node) && node.variableDeclaration)
    add(node.variableDeclaration.name);
  if (
    (ts.isForStatement(node) ||
      ts.isForInStatement(node) ||
      ts.isForOfStatement(node)) &&
    node.initializer &&
    ts.isVariableDeclarationList(node.initializer)
  )
    for (const declaration of node.initializer.declarations)
      add(declaration.name);
  const statements =
    ts.isBlock(node) || ts.isSourceFile(node) || ts.isModuleBlock(node)
      ? node.statements
      : ts.isCaseBlock(node)
        ? node.clauses.flatMap((clause) => [...clause.statements])
        : [];
  for (const statement of statements) {
    if (ts.isVariableStatement(statement))
      for (const declaration of statement.declarationList.declarations)
        add(declaration.name);
    else if (
      (ts.isFunctionDeclaration(statement) ||
        ts.isClassDeclaration(statement) ||
        ts.isEnumDeclaration(statement)) &&
      statement.name
    )
      names.push(statement.name);
    else if (ts.isImportEqualsDeclaration(statement))
      names.push(statement.name);
    else if (ts.isImportDeclaration(statement) && statement.importClause) {
      const clause = statement.importClause;
      if (clause.name) names.push(clause.name);
      const bindings = clause.namedBindings;
      if (bindings && ts.isNamespaceImport(bindings)) names.push(bindings.name);
      else if (bindings)
        for (const element of bindings.elements) names.push(element.name);
    }
  }
  return names;
}

/** The declaration a name resolves to, or undefined for a global. */
function declarationOf(reference: ts.Identifier): ts.Identifier | undefined {
  for (
    let node: ts.Node | undefined = reference.parent;
    node;
    node = node.parent
  ) {
    const declared = declaredNames(node).find(
      (name) => name.text === reference.text,
    );
    if (declared) return declared;
  }
  return undefined;
}

/** The node whose code sees a declared name. */
function scopeOf(declared: ts.Identifier): ts.Node {
  let declaration: ts.Node = declared.parent;
  while (
    ts.isBindingElement(declaration) ||
    ts.isObjectBindingPattern(declaration) ||
    ts.isArrayBindingPattern(declaration)
  )
    declaration = declaration.parent;
  if (ts.isParameter(declaration)) return declaration.parent;
  if (ts.isVariableDeclaration(declaration)) {
    if (ts.isCatchClause(declaration.parent)) return declaration.parent;
    const list = declaration.parent;
    const blockScoped = (list.flags & ts.NodeFlags.BlockScoped) !== 0;
    for (let node: ts.Node = list.parent; ; node = node.parent) {
      if (ts.isSourceFile(node) || ts.isFunctionLike(node)) return node;
      if (
        blockScoped &&
        (ts.isBlock(node) ||
          ts.isModuleBlock(node) ||
          ts.isCaseBlock(node) ||
          ts.isForStatement(node) ||
          ts.isForInStatement(node) ||
          ts.isForOfStatement(node))
      )
        return node;
    }
  }
  return declared.getSourceFile();
}

/** Nodes whose `name` declares something rather than reads a value. */
const DECLARATIONS: ((node: ts.Node) => boolean)[] = [
  ts.isVariableDeclaration,
  ts.isParameter,
  ts.isBindingElement,
  ts.isFunctionDeclaration,
  ts.isFunctionExpression,
  ts.isClassDeclaration,
  ts.isClassExpression,
  ts.isImportClause,
  ts.isNamespaceImport,
  ts.isImportSpecifier,
  ts.isImportEqualsDeclaration,
  ts.isNamespaceExport,
  ts.isTypeAliasDeclaration,
  ts.isInterfaceDeclaration,
  ts.isTypeParameterDeclaration,
  ts.isEnumDeclaration,
  ts.isEnumMember,
  ts.isModuleDeclaration,
  ts.isPropertyAssignment,
  ts.isPropertyDeclaration,
  ts.isPropertySignature,
  ts.isMethodDeclaration,
  ts.isMethodSignature,
  ts.isGetAccessorDeclaration,
  ts.isSetAccessorDeclaration,
];

/** Whether an identifier reads a value, not names a property or a type. */
function isReference(id: ts.Identifier): boolean {
  const parent = id.parent;
  if (ts.isPropertyAccessExpression(parent)) return parent.expression === id;
  if (ts.isQualifiedName(parent) || ts.isMetaProperty(parent)) return false;
  if (ts.isBindingElement(parent) && parent.propertyName === id) return false;
  if (ts.isImportSpecifier(parent) && parent.propertyName === id) return false;
  if (ts.isExportSpecifier(parent))
    return !parent.propertyName || parent.propertyName === id;
  if (ts.isLabeledStatement(parent) || ts.isBreakOrContinueStatement(parent))
    return false;
  if (ts.isJsxAttribute(parent)) return parent.name !== id;
  if (DECLARATIONS.some((is) => is(parent)))
    return (parent as ts.NamedDeclaration).name !== id;
  if (ts.isTypeNode(parent) && !ts.isExpressionWithTypeArguments(parent))
    return false;
  return true;
}

/** Every reference to a declared name inside its scope, shadowing respected. */
function references(declared: ts.Identifier): ts.Identifier[] {
  const scope = scopeOf(declared);
  const found: ts.Identifier[] = [];
  walk(scope, (node) => {
    if (
      !ts.isIdentifier(node) ||
      node === declared ||
      node.text !== declared.text ||
      !isReference(node)
    )
      return;
    for (let up = node.parent; up !== scope; up = up.parent)
      if (
        declaredNames(up).some(
          (name) =>
            name !== declared &&
            name.text === node.text &&
            scopeOf(name) !== scope,
        )
      )
        return;
    found.push(node);
  });
  return found;
}

function isExported(declared: ts.Identifier): boolean {
  let node: ts.Node = declared.parent;
  while (
    ts.isBindingElement(node) ||
    ts.isObjectBindingPattern(node) ||
    ts.isArrayBindingPattern(node) ||
    ts.isVariableDeclaration(node) ||
    ts.isVariableDeclarationList(node)
  )
    node = node.parent;
  return (
    ts.canHaveModifiers(node) &&
    (ts.getModifiers(node) ?? []).some(
      (modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword,
    )
  );
}

function isModuleExportsTarget(node: ts.Expression): boolean {
  return /^(module\.)?exports\b/.test(node.getText());
}

class Inventory {
  readonly writes: CookieWrite[] = [];
  private readonly seen = new Set<string>();
  /** cookies of next/headers, kept from the cookie list seeds. */
  private readonly jarFactories = new Set<ts.Node>();
  /** Local names of headers() of next/headers. */
  private readonly headersFactories = new Set<string>();

  private readonly file: ts.SourceFile;

  constructor(file: ts.SourceFile) {
    this.file = file;
  }

  run(): void {
    walk(this.file, (node) => this.seedModule(node));
    walk(this.file, (node) => this.seedValue(node));
  }

  reportExport(node: ts.Node): void {
    this.report("module-export", node);
  }

  private report(kind: CookieWriteKind, node: ts.Node): void {
    const line =
      this.file.getLineAndCharacterOfPosition(node.getStart(this.file)).line +
      1;
    if (this.writes.some((write) => write.kind === kind && write.line === line))
      return;
    const code = node.getText(this.file).replace(/\s+/g, " ").slice(0, 80);
    this.writes.push({ kind, line, code });
  }

  private once(node: ts.Node, value: Value, what: string): boolean {
    const key = `${what}:${node.pos}:${node.end}:${node.kind}:${value}`;
    if (this.seen.has(key)) return false;
    this.seen.add(key);
    return true;
  }

  /** next/headers, cookie libraries and Next's cookie list classes. */
  private seedModule(node: ts.Node): void {
    if (
      ts.isImportDeclaration(node) &&
      ts.isStringLiteral(node.moduleSpecifier)
    ) {
      const specifier = node.moduleSpecifier.text;
      const clause = node.importClause;
      if (!clause || clause.phaseModifier === ts.SyntaxKind.TypeKeyword) return;
      if (isCookieLibrary(specifier)) this.report("cookie-library", node);
      const bindings = clause.namedBindings;
      if (specifier === "next/headers") {
        if (clause.name) this.bindIdentifier(clause.name, "headers-module");
        if (bindings && ts.isNamespaceImport(bindings))
          this.bindIdentifier(bindings.name, "headers-module");
      }
      if (bindings && ts.isNamedImports(bindings))
        for (const element of bindings.elements) {
          if (element.isTypeOnly) continue;
          const imported = (element.propertyName ?? element.name).text;
          if (specifier === "next/headers" && imported === "headers")
            this.headersFactories.add(element.name.text);
          if (specifier === "next/headers" && imported === "cookies") {
            this.jarFactories.add(element.name);
            this.bindIdentifier(element.name, "jar-factory");
          }
          if (LIST_FACTORIES.has(imported))
            this.bindIdentifier(element.name, "list-factory");
        }
      return;
    }
    if (ts.isExportDeclaration(node) && node.moduleSpecifier) {
      if (!ts.isStringLiteral(node.moduleSpecifier) || node.isTypeOnly) return;
      const specifier = node.moduleSpecifier.text;
      if (isCookieLibrary(specifier)) this.report("cookie-library", node);
      const clause = node.exportClause;
      if (
        specifier === "next/headers" &&
        (!clause ||
          ts.isNamespaceExport(clause) ||
          clause.elements.some(
            (element) =>
              !element.isTypeOnly &&
              (element.propertyName ?? element.name).text === "cookies",
          ))
      )
        this.report("module-export", node);
      return;
    }
    if (
      ts.isImportEqualsDeclaration(node) &&
      ts.isExternalModuleReference(node.moduleReference) &&
      ts.isStringLiteral(node.moduleReference.expression)
    ) {
      const specifier = node.moduleReference.expression.text;
      if (isCookieLibrary(specifier)) this.report("cookie-library", node);
      if (specifier === "next/headers")
        this.bindIdentifier(node.name, "headers-module");
      return;
    }
    if (ts.isCallExpression(node)) {
      const loads =
        node.expression.kind === ts.SyntaxKind.ImportKeyword ||
        (ts.isIdentifier(node.expression) &&
          node.expression.text === "require");
      const specifier = node.arguments[0] && stringValue(node.arguments[0]);
      if (!loads || specifier === undefined) return;
      if (isCookieLibrary(specifier)) this.report("cookie-library", node);
      if (specifier === "next/headers") this.track(node, "headers-module");
    }
  }

  /** Cookie lists, document, cookieStore and Set-Cookie header names. */
  private seedValue(node: ts.Node): void {
    if (
      ts.isElementAccessExpression(node) &&
      memberName(node) === undefined &&
      (this.isMessage(node.expression, 0) || isBeforeWriteMethod(node))
    )
      this.report("computed-member", node);
    const member = memberName(node);
    if (member !== undefined) {
      const written = isWriteTarget(node);
      const expression = node as ts.Expression;
      // `delete document.cookie` changes nothing; a header map's Cookie
      // entry is deleted the same way.
      if (member === "cookie" && written && !isDeleted(node))
        this.report("document-cookie", node);
      if (written) return;
      if (member === "cookies" && !this.jarFactories.has(node))
        this.track(expression, "list");
      if (member === "cookieStore") this.track(expression, "list");
      if (DOCUMENT_MEMBERS.has(member)) this.track(expression, "document");
      if (member === "contentWindow") this.track(expression, "global");
      if (LIST_FACTORIES.has(member)) this.track(expression, "list-factory");
      return;
    }
    if (ts.isBindingElement(node) && !this.jarFactories.has(node)) {
      const property = boundProperty(node);
      if (property === "cookies" || property === "cookieStore")
        this.bind(node.name, "list");
      return;
    }
    if (
      ts.isIdentifier(node) &&
      (node.text === "document" ||
        node.text === "cookieStore" ||
        GLOBALS.has(node.text)) &&
      isReference(node) &&
      !declarationOf(node)
    ) {
      this.track(
        node,
        node.text === "document"
          ? "document"
          : node.text === "cookieStore"
            ? "list"
            : "global",
      );
      return;
    }
    const text = spelledText(node);
    if (text === undefined || !SET_COOKIE.test(text)) return;
    this.report("cookie-header", node);
    if (
      ts.isTemplateHead(node) ||
      ts.isTemplateMiddle(node) ||
      ts.isTemplateTail(node)
    ) {
      let template: ts.Node | undefined = node.parent;
      // Type-level templates share these tokens but carry no runtime value.
      while (
        template &&
        !ts.isTemplateExpression(template) &&
        !ts.isTemplateLiteralTypeNode(template)
      )
        template = template.parent;
      if (template && ts.isTemplateExpression(template))
        this.track(template, "cookie-header-name");
    } else if (ts.isExpression(node)) this.track(node, "cookie-header-name");
  }

  /**
   * Whether an expression holds a request, a response or headers, read from
   * the syntax: a constructor or factory, fetch, headers() of next/headers,
   * an annotation or a conventional name. A cookie list or the jar reports a
   * computed member itself.
   */
  private isMessage(expression: ts.Expression, depth: number): boolean {
    let node: ts.Node = expression;
    while (
      ts.isParenthesizedExpression(node) ||
      ts.isAwaitExpression(node) ||
      ts.isAsExpression(node) ||
      ts.isNonNullExpression(node) ||
      ts.isSatisfiesExpression(node)
    )
      node = node.expression;
    if (ts.isCallExpression(node) || ts.isNewExpression(node)) {
      const callee = node.expression;
      if (ts.isPropertyAccessExpression(callee))
        return (
          callee.name.text === "headers" ||
          MESSAGE_CLASSES.has(callee.expression.getText())
        );
      return (
        ts.isIdentifier(callee) &&
        (MESSAGE_CLASSES.has(callee.text) ||
          callee.text === "fetch" ||
          this.headersFactories.has(callee.text))
      );
    }
    if (!ts.isIdentifier(node)) return false;
    if (MESSAGE_NAMES.test(node.text)) return true;
    const declared = declarationOf(node);
    const declaration = declared?.parent;
    if (
      !declaration ||
      !(ts.isParameter(declaration) || ts.isVariableDeclaration(declaration)) ||
      declaration.name !== declared
    )
      return false;
    if (declaration.type && isMessageType(declaration.type)) return true;
    return (
      depth < 8 &&
      declaration.initializer !== undefined &&
      this.isMessage(declaration.initializer, depth + 1)
    );
  }

  /** Follows the value up through wrappers that hand it on unchanged. */
  private track(node: ts.Expression, value: Value): void {
    if (!this.once(node, value, "track")) return;
    let current: ts.Node = node;
    for (;;) {
      const parent = current.parent;
      if (
        ts.isParenthesizedExpression(parent) ||
        ts.isAwaitExpression(parent) ||
        ts.isAsExpression(parent) ||
        ts.isTypeAssertionExpression(parent) ||
        ts.isNonNullExpression(parent) ||
        ts.isSatisfiesExpression(parent) ||
        (ts.isConditionalExpression(parent) && parent.condition !== current)
      ) {
        current = parent;
        continue;
      }
      if (ts.isBinaryExpression(parent)) {
        const operator = parent.operatorToken.kind;
        if (
          operator === ts.SyntaxKind.QuestionQuestionToken ||
          operator === ts.SyntaxKind.BarBarToken ||
          (parent.right === current &&
            (operator === ts.SyntaxKind.AmpersandAmpersandToken ||
              operator === ts.SyntaxKind.CommaToken))
        ) {
          current = parent;
          continue;
        }
      }
      break;
    }
    this.use(current, value);
  }

  /** Every use that is not a known read or alias counts as a write. */
  private use(node: ts.Node, value: Value): void {
    const parent = node.parent;
    if (
      value === "global" &&
      !(
        (ts.isPropertyAccessExpression(parent) ||
          ts.isElementAccessExpression(parent)) &&
        parent.expression === node
      ) &&
      !(ts.isVariableDeclaration(parent) && parent.initializer === node)
    )
      // window goes everywhere; code that reaches document.cookie through it
      // assigns `.cookie`, which counts on any object.
      return;
    if (
      (ts.isPropertyAccessExpression(parent) ||
        ts.isElementAccessExpression(parent)) &&
      parent.expression === node
    ) {
      const name = memberName(parent);
      if (name !== undefined) this.member(parent, name, value);
      // A computed key on window is a read; a write through what it returns
      // assigns `.cookie` again.
      else if (value !== "global") this.report(KIND[value], parent);
      return;
    }
    if (ts.isCallExpression(parent) && parent.expression === node) {
      if (value === "jar-factory") this.track(parent, "jar");
      else this.report(KIND[value], parent);
      return;
    }
    if (ts.isNewExpression(parent) && parent.expression === node) {
      if (value === "list-factory") this.track(parent, "list");
      else this.report(KIND[value], parent);
      return;
    }
    if (
      (ts.isVariableDeclaration(parent) ||
        ts.isBindingElement(parent) ||
        ts.isParameter(parent)) &&
      parent.initializer === node
    ) {
      this.bind(parent.name, value);
      return;
    }
    if (
      ts.isBinaryExpression(parent) &&
      isAssignment(parent.operatorToken.kind)
    ) {
      if (parent.left === node) return;
      const target = parent.left;
      if (
        parent.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
        ts.isIdentifier(target)
      ) {
        const declared = declarationOf(target);
        if (declared) this.bindIdentifier(declared, value);
        else this.report(KIND[value], parent);
        this.track(parent, value);
        return;
      }
      this.report(
        isModuleExportsTarget(target) ? "module-export" : KIND[value],
        parent,
      );
      return;
    }
    if (ts.isExportSpecifier(parent) || ts.isExportAssignment(parent)) {
      this.report("module-export", parent);
      return;
    }
    if (
      isCondition(node) ||
      ts.isExpressionStatement(parent) ||
      ts.isVoidExpression(parent)
    )
      return;
    this.report(KIND[value], parent);
  }

  private member(node: ts.Node, name: string, value: Value): void {
    switch (value) {
      case "headers-module":
        if (name === "cookies") {
          this.jarFactories.add(node);
          this.track(node as ts.Expression, "jar-factory");
        }
        return;
      case "jar":
        if (isWriteTarget(node) || !READS.has(name))
          this.report(KIND.jar, node);
        return;
      case "list":
        if (isWriteTarget(node) || !LIST_READS.has(name))
          this.report(KIND.list, node);
        return;
      case "global":
        if (name === "document") this.track(node as ts.Expression, "document");
        if (name === "cookieStore") this.track(node as ts.Expression, "list");
        return;
      case "document":
        if (name === "cookie" && isWriteTarget(node) && !isDeleted(node))
          this.report(KIND.document, node);
        return;
      default:
        this.report(KIND[value], node);
    }
  }

  private bind(name: ts.BindingName, value: Value): void {
    if (ts.isIdentifier(name)) {
      this.bindIdentifier(name, value);
      return;
    }
    if (ts.isArrayBindingPattern(name)) {
      this.report(KIND[value], name);
      return;
    }
    for (const element of name.elements) {
      const property = boundProperty(element);
      if (element.dotDotDotToken || property === undefined) {
        this.report(KIND[value], element);
        continue;
      }
      switch (value) {
        case "headers-module":
          if (property === "cookies") {
            this.jarFactories.add(element);
            this.bind(element.name, "jar-factory");
          }
          break;
        case "jar":
          if (!READS.has(property)) this.report(KIND.jar, element);
          break;
        case "list":
          if (!LIST_READS.has(property)) this.report(KIND.list, element);
          break;
        case "global":
          if (property === "document") this.bind(element.name, "document");
          if (property === "cookieStore") this.bind(element.name, "list");
          break;
        case "document":
          // A destructured property is a read; `cookie` reads the string.
          break;
        default:
          this.report(KIND[value], element);
      }
    }
  }

  private bindIdentifier(declared: ts.Identifier, value: Value): void {
    if (!this.once(declared, value, "bind")) return;
    if (isExported(declared)) this.report("module-export", declared);
    for (const reference of references(declared)) this.track(reference, value);
  }
}

export interface CookieInventory {
  /** The ways the file can write a cookie that the inventory recognises. */
  writes: CookieWrite[];
  /**
   * Exports that carry a Set-Cookie or session cookie name, also nested in
   * objects, arrays, returns, and constants or functions they reference: an
   * importer could write the cookie without spelling its name.
   */
  exportedCookieNames: { line: number; code: string }[];
  /**
   * Every place the file names or reads a session cookie name: a spelled
   * `session-token`, also in a regular expression, or Auth.js' `sessionToken`
   * cookie options.
   */
  sessionCookieNames: { line: number; code: string }[];
}

/** Calls that fill their receiver. */
const FILLING_METHODS = new Set([
  "push",
  "unshift",
  "splice",
  "set",
  "add",
  "append",
]);

function rootIdentifier(expression: ts.Expression): ts.Identifier | undefined {
  let node: ts.Expression = expression;
  for (;;) {
    if (
      ts.isParenthesizedExpression(node) ||
      ts.isAsExpression(node) ||
      ts.isNonNullExpression(node) ||
      ts.isSatisfiesExpression(node) ||
      ts.isPropertyAccessExpression(node) ||
      ts.isElementAccessExpression(node)
    )
      node = node.expression;
    else return ts.isIdentifier(node) ? node : undefined;
  }
}

/**
 * Bindings a later write fills with a cookie name: a property assignment,
 * Object.assign, Object.defineProperty, Reflect.set or a filling call.
 */
function filledWithCookieName(sourceFile: ts.SourceFile): Set<ts.Node> {
  const filled = new Set<ts.Node>();
  const fill = (target: ts.Expression | undefined, values: ts.Node[]) => {
    const root = target && rootIdentifier(target);
    const declared = root && declarationOf(root);
    // Only module bindings can leave the module; a local one stays inside
    // the function whose own text the export check reads.
    if (
      declared &&
      ts.isSourceFile(scopeOf(declared)) &&
      values.some((value) => carriesCookieName(value, new Set()))
    )
      filled.add(declared);
  };
  walk(sourceFile, (node) => {
    if (
      ts.isBinaryExpression(node) &&
      isAssignment(node.operatorToken.kind) &&
      (ts.isPropertyAccessExpression(node.left) ||
        ts.isElementAccessExpression(node.left))
    )
      fill(node.left, [node.right]);
    if (!ts.isCallExpression(node)) return;
    const callee = node.expression;
    if (!ts.isPropertyAccessExpression(callee)) return;
    const owner = callee.expression.getText();
    const [first, ...rest] = node.arguments;
    if (
      (owner === "Object" &&
        ["assign", "defineProperty", "defineProperties"].includes(
          callee.name.text,
        )) ||
      (owner === "Reflect" && callee.name.text === "set")
    )
      fill(first, rest);
    else if (FILLING_METHODS.has(callee.name.text))
      fill(callee.expression, [...node.arguments]);
  });
  return filled;
}

/** Bindings filled later, and whether a check reached one. */
interface Filled {
  bindings: Set<ts.Node>;
  reached: boolean;
}

/** Whether a node spells, matches or reaches a cookie name in this file. */
function carriesCookieName(
  node: ts.Node,
  seen: Set<ts.Node>,
  filled?: Filled,
): boolean {
  const text = ts.isRegularExpressionLiteral(node)
    ? node.text
    : spelledText(node);
  if (text !== undefined && (SET_COOKIE.test(text) || SESSION_TOKEN.test(text)))
    return true;
  if (ts.isIdentifier(node) && isReference(node)) {
    const declared = declarationOf(node);
    if (declared && filled?.bindings.has(declared)) {
      filled.reached = true;
      return true;
    }
    if (declared && !seen.has(declared)) {
      seen.add(declared);
      let declaration: ts.Node = declared.parent;
      // A binding can take its value from a nested default or the whole
      // source initializer. Follow both conservatively, including arrays.
      while (
        ts.isBindingElement(declaration) ||
        ts.isObjectBindingPattern(declaration) ||
        ts.isArrayBindingPattern(declaration)
      ) {
        if (
          ts.isBindingElement(declaration) &&
          declaration.initializer &&
          carriesCookieName(declaration.initializer, seen, filled)
        )
          return true;
        declaration = declaration.parent;
      }
      if (
        ts.isVariableDeclaration(declaration) &&
        declaration.initializer &&
        carriesCookieName(declaration.initializer, seen, filled)
      )
        return true;
      if (
        (ts.isFunctionDeclaration(declaration) ||
          ts.isClassDeclaration(declaration)) &&
        carriesCookieName(declaration, seen, filled)
      )
        return true;
    }
  }
  return (
    ts.forEachChild(
      node,
      (child) => carriesCookieName(child, seen, filled) || undefined,
    ) ?? false
  );
}

/** `NextRequest`, `Response | null`, `Promise<NextResponse>` … */
function isMessageType(type: ts.TypeNode): boolean {
  if (ts.isParenthesizedTypeNode(type)) return isMessageType(type.type);
  if (ts.isUnionTypeNode(type) || ts.isIntersectionTypeNode(type))
    return type.types.some(isMessageType);
  if (!ts.isTypeReferenceNode(type)) return false;
  const name = ts.isIdentifier(type.typeName)
    ? type.typeName.text
    : type.typeName.right.text;
  return (
    MESSAGE_TYPES.has(name) ||
    (name === "Promise" && (type.typeArguments ?? []).some(isMessageType))
  );
}

function hasExportModifier(node: ts.Node): boolean {
  return (
    ts.canHaveModifiers(node) &&
    (ts.getModifiers(node) ?? []).some(
      (modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword,
    )
  );
}

/**
 * Exports that carry a cookie name; `filled` lists those that reach a binding
 * a later write filled with one.
 */
function exportedCookieNames(sourceFile: ts.SourceFile): {
  found: { line: number; code: string }[];
  filled: ts.Node[];
} {
  const found: { line: number; code: string }[] = [];
  const filledExports: ts.Node[] = [];
  const bindings = filledWithCookieName(sourceFile);
  const check = (at: ts.Node, ...values: (ts.Node | undefined)[]) => {
    const filled: Filled = { bindings, reached: false };
    if (values.some((value) => value && bindings.has(value)))
      filled.reached = true;
    else if (
      !values.some(
        (value) => value && carriesCookieName(value, new Set(), filled),
      )
    )
      return;
    if (filled.reached) filledExports.push(at);
    const line =
      sourceFile.getLineAndCharacterOfPosition(at.getStart(sourceFile)).line +
      1;
    if (found.some((entry) => entry.line === line)) return;
    found.push({
      line,
      code: at.getText(sourceFile).replace(/\s+/g, " ").slice(0, 80),
    });
  };
  for (const statement of sourceFile.statements) {
    if (ts.isVariableStatement(statement) && hasExportModifier(statement))
      for (const declaration of statement.declarationList.declarations)
        // The name too: a write after the declaration fills the binding.
        check(declaration, declaration.name, declaration.initializer);
    else if (
      (ts.isFunctionDeclaration(statement) ||
        ts.isClassDeclaration(statement)) &&
      hasExportModifier(statement)
    )
      check(statement, statement);
    else if (ts.isExportAssignment(statement))
      check(statement, statement.expression);
    else if (
      ts.isExportDeclaration(statement) &&
      !statement.moduleSpecifier &&
      !statement.isTypeOnly &&
      statement.exportClause &&
      ts.isNamedExports(statement.exportClause)
    )
      for (const element of statement.exportClause.elements)
        if (!element.isTypeOnly)
          check(element, element.propertyName ?? element.name);
  }
  walk(sourceFile, (node) => {
    if (
      ts.isBinaryExpression(node) &&
      node.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
      isModuleExportsTarget(node.left)
    )
      check(node, node.right);
  });
  return {
    found: found.sort((a, b) => a.line - b.line),
    filled: filledExports,
  };
}

/** Both inventories of one file, from one parse of its syntax tree. */
export function inventoryOf(file: string, source: string): CookieInventory {
  const sourceFile = parse(file, source);
  const inventory = new Inventory(sourceFile);
  inventory.run();
  const sessionCookieNames: { line: number; code: string }[] = [];
  walk(sourceFile, (node) => {
    const text = ts.isRegularExpressionLiteral(node)
      ? node.text
      : spelledText(node);
    const named =
      (text !== undefined && SESSION_TOKEN.test(text)) ||
      memberName(node) === "sessionToken" ||
      (ts.isBindingElement(node) && boundProperty(node) === "sessionToken");
    if (!named) return;
    const line =
      sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line +
      1;
    if (sessionCookieNames.some((entry) => entry.line === line)) return;
    sessionCookieNames.push({
      line,
      code: node.getText(sourceFile).slice(0, 80),
    });
  });
  const exported = exportedCookieNames(sourceFile);
  // A cookie name an export receives only through a later write leaves the
  // module unlisted, past the per-file writer check.
  for (const at of exported.filled) inventory.reportExport(at);
  return {
    writes: inventory.writes.sort(
      (a, b) => a.line - b.line || a.kind.localeCompare(b.kind),
    ),
    sessionCookieNames,
    exportedCookieNames: exported.found,
  };
}

/** The cookie writes the inventory recognises, from the syntax tree. */
export function cookieWrites(file: string, source: string): CookieWrite[] {
  return inventoryOf(file, source).writes;
}

/** See CookieInventory.sessionCookieNames. */
export function sessionCookieNameSources(
  file: string,
  source: string,
): { line: number; code: string }[] {
  return inventoryOf(file, source).sessionCookieNames;
}

/** What the inventory reads: the app, the browser's static scripts, Next's config. */
export const INVENTORY_ROOTS = ["src", "public", "next.config.js"];

/** Source files of the inventory, relative to `root`. */
export function inventoryFiles(root: string, entries: string[]): string[] {
  const visit = (absolute: string): string[] => {
    if (statSync(absolute).isDirectory())
      return readdirSync(absolute).flatMap((entry) =>
        visit(path.join(absolute, entry)),
      );
    if (
      !/\.[cm]?[jt]sx?$/.test(absolute) ||
      /\.(test|spec)\.[cm]?[jt]sx?$/.test(absolute)
    )
      return [];
    return [path.relative(root, absolute)];
  };
  return entries.flatMap((entry) => visit(path.join(root, entry))).sort();
}
