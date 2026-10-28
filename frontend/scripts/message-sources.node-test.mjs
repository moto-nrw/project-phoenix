import assert from "node:assert/strict";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import ts from "typescript";
import {
  assertLocaleDirectories,
  assertMessageParity,
  composeMessages,
  readMessageSources,
} from "./message-sources.mjs";
import { synchronizeMessageFacades } from "./generate-message-index.mjs";

async function fixture(run) {
  const directory = mkdtempSync(join(tmpdir(), "moto-message-sources-"));
  try {
    await run(directory);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

function writeSource(directory, relative, value) {
  const file = join(directory, relative);
  mkdirSync(join(file, ".."), { recursive: true });
  writeFileSync(file, JSON.stringify(value));
}

test("nested domain fragments compose the original message shape without losing values", () =>
  fixture((directory) => {
    writeSource(directory, "errorCatalog/codes/care.json", {
      child_required: "Bitte wählen Sie ein Kind.",
    });
    writeSource(directory, "errorCatalog/classes.json", {
      input: "Bitte prüfen Sie {object}.",
    });
    writeSource(directory, "parentChild.json", {
      consents: { title: "Einverständnisse", choices: ["Ja", "Nein"] },
    });
    assert.deepEqual(composeMessages(readMessageSources(directory)), {
      errorCatalog: {
        classes: { input: "Bitte prüfen Sie {object}." },
        codes: { care: { child_required: "Bitte wählen Sie ein Kind." } },
      },
      parentChild: {
        consents: { title: "Einverständnisse", choices: ["Ja", "Nein"] },
      },
    });
  }));

test("parent/child and duplicate source mounts fail regardless of discovery order", () => {
  const parent = {
    path: ["errorCatalog"],
    file: "errorCatalog.json",
    value: {},
  };
  const child = {
    path: ["errorCatalog", "codes", "care"],
    file: "errorCatalog/codes/care.json",
    value: {},
  };
  for (const sources of [
    [parent, child],
    [child, parent],
    [parent, parent],
  ])
    assert.throws(() => composeMessages(sources), /Overlapping/);
});

test("duplicate JSON properties, including escaped spellings, fail before parsing can hide them", () =>
  fixture((directory) => {
    writeFileSync(
      join(directory, "labels.json"),
      '{"nested":{"name":"first","na\\u006de":"second"}}',
    );
    assert.throws(
      () => readMessageSources(directory),
      /duplicate JSON key name/,
    );
  }));

test("unknown/missing locale directories and missing fallback catalogs fail closed", () =>
  fixture((directory) => {
    mkdirSync(join(directory, "de"));
    assert.throws(
      () => assertLocaleDirectories(directory, ["de", "en"]),
      /supported locales/,
    );
    mkdirSync(join(directory, "unexpected"));
    assert.throws(
      () => assertLocaleDirectories(directory, ["de"]),
      /supported locales/,
    );
    assert.throws(
      () => assertMessageParity(new Map([["en", {}]]), "de"),
      /Missing fallback/,
    );
  }));

test("global leaf parity still catches missing and extra keys across split namespaces/domains", () => {
  const german = {
    errorCatalog: { codes: { care: { child_required: "Kind" } } },
    labels: { title: "Titel" },
  };
  assert.doesNotThrow(() =>
    assertMessageParity(
      new Map([
        ["de", german],
        ["en", german],
      ]),
      "de",
    ),
  );
  assert.throws(
    () =>
      assertMessageParity(
        new Map([
          ["de", german],
          ["en", { labels: { title: "Title", extra: "Extra" } }],
        ]),
        "de",
      ),
    /missing keys.*care.child_required[\s\S]*unexpected keys.*labels.extra/,
  );
});

test("new fragments are automatically indexed and stale static facades cannot pass", () =>
  fixture(async (directory) => {
    writeSource(directory, "de/labels.json", { title: "Titel" });
    await synchronizeMessageFacades({ directory, locales: ["de"] });
    await synchronizeMessageFacades({
      directory,
      locales: ["de"],
      check: true,
    });
    const before = readFileSync(join(directory, "de.generated.ts"), "utf8");
    writeSource(directory, "de/newNamespace.json", { title: "Neu" });
    await assert.rejects(
      () =>
        synchronizeMessageFacades({ directory, locales: ["de"], check: true }),
      /stale/,
    );
    await synchronizeMessageFacades({ directory, locales: ["de"] });
    const after = readFileSync(join(directory, "de.generated.ts"), "utf8");
    for (const line of before
      .split("\n")
      .filter((line) => line.startsWith("import ")))
      assert.ok(
        after.includes(line),
        "unrelated additions must not rename existing imports",
      );
    assert.match(after, /newNamespace/);
    assert.doesNotMatch(after, /node:fs|import\.meta\.glob|Record<string/);
  }));

test("generated static facades retain exact nested TypeScript inference", () =>
  fixture(async (directory) => {
    writeSource(directory, "de/parentChild.json", {
      consents: { title: "Einverständnisse" },
    });
    writeSource(directory, "de/errorCatalog/codes/care.json", {
      child_required: "Kind wählen",
    });
    await synchronizeMessageFacades({ directory, locales: ["de"] });
    const contract = join(directory, "contract.ts");
    writeFileSync(
      contract,
      `import messages from './de.generated';
const title: string = messages.parentChild.consents.title;
const error: string = messages.errorCatalog.codes.care.child_required;
// @ts-expect-error unknown nested keys must stay compile errors
messages.parentChild.consents.unknown;
// @ts-expect-error unknown domains must stay compile errors
messages.errorCatalog.codes.unknown;
export { title, error };
`,
    );
    const program = ts.createProgram([contract], {
      noEmit: true,
      strict: true,
      resolveJsonModule: true,
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
      moduleResolution: ts.ModuleResolutionKind.Bundler,
      esModuleInterop: true,
      types: [],
      skipLibCheck: true,
      lib: ["lib.es2022.d.ts"],
    });
    const diagnostics = ts.getPreEmitDiagnostics(program);
    assert.deepEqual(
      diagnostics.map((diagnostic) =>
        ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n"),
      ),
      [],
    );
  }));
