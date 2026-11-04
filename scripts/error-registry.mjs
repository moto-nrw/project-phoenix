import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

export const ERROR_CLASSES = ["input", "permission", "business_rejection", "unavailable", "server"];
const codePattern = /^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$/;
const parameterPattern = /^[a-z][a-z0-9_]*$/;

export function goSymbol(code) {
  return "Code" + code.split(/[._]/).map((part) => part[0].toUpperCase() + part.slice(1)).join("");
}

export function validateRegistry(registry) {
  if (!registry || !Array.isArray(registry.codes)) throw new Error("registry.codes must be an array");
  const codes = new Set();
  const legacy = new Set();
  const symbols = new Set();
  for (const entry of registry.codes) {
    if (!codePattern.test(entry.code)) throw new Error(`invalid error code: ${entry.code}`);
    if (codes.has(entry.code)) throw new Error(`duplicate error code: ${entry.code}`);
    codes.add(entry.code);
    if (!ERROR_CLASSES.includes(entry.class)) throw new Error(`invalid class for ${entry.code}: ${entry.class}`);
    if (!Array.isArray(entry.parameters)) throw new Error(`parameters must be an array: ${entry.code}`);
    const parameters = new Set();
    for (const parameter of entry.parameters) {
      if (!parameterPattern.test(parameter) || parameters.has(parameter)) {
        throw new Error(`invalid or duplicate parameter for ${entry.code}: ${parameter}`);
      }
      parameters.add(parameter);
    }
    if (typeof entry.legacy !== "string" || !entry.legacy) throw new Error(`missing legacy code: ${entry.code}`);
    if (legacy.has(entry.legacy)) throw new Error(`duplicate legacy code: ${entry.legacy}`);
    legacy.add(entry.legacy);
    const symbol = goSymbol(entry.code);
    if (symbols.has(symbol)) throw new Error(`duplicate Go symbol: ${symbol}`);
    symbols.add(symbol);
  }
  for (const entry of registry.codes) {
    if (entry.legacy !== entry.code && codes.has(entry.legacy)) {
      throw new Error(`legacy code shadows another canonical code: ${entry.legacy}`);
    }
  }
  return [...registry.codes].sort((a, b) => a.code.localeCompare(b.code, "en"));
}

/** Discover sources, then validate the complete registry, never each domain alone. */
export function readRegistry(root = fileURLToPath(new URL("../", import.meta.url))) {
  const directory = join(root, "error-registry");
  const files = readdirSync(directory).filter((name) => name.endsWith(".json")).sort();
  if (!files.length) throw new Error(`${directory}: no error registry fragments`);
  const codes = [];
  for (const name of files) {
    if (!/^[a-z][a-z0-9_]*\.json$/.test(name)) throw new Error(`invalid registry fragment: ${name}`);
    const fragment = JSON.parse(readFileSync(join(directory, name), "utf8"));
    const entries = validateRegistry(fragment);
    if (!entries.length) throw new Error(`${name}: empty error registry fragment`);
    for (const entry of entries) {
      if (entry.code.split(".")[0] !== name.slice(0, -5)) {
        throw new Error(`${name}: wrong domain for ${entry.code}`);
      }
    }
    codes.push(...entries);
  }
  return { codes: validateRegistry({ codes }) };
}
