import {
  constants,
  copyFileSync,
  cpSync,
  mkdirSync,
  mkdtempSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { pathToFileURL } from "node:url";

/**
 * Next also writes next-env.d.ts and tsconfig.json in the app root. A separate
 * distDir alone would still mutate the native dev loop's generated files.
 */
export function createAppWorkspace(frontendDirectory) {
  // Keep snapshots outside frontend: its test/type/source scanners must never
  // discover a second copy of the application while an E2E run is active.
  const repositoryDirectory = dirname(frontendDirectory);
  const parent = join(repositoryDirectory, "tmp", "network-e2e");
  mkdirSync(parent, { recursive: true });
  const directory = mkdtempSync(join(parent, "run-"));
  const cleanup = () => rmSync(directory, { recursive: true, force: true });
  try {
    for (const name of [
      "package.json",
      "postcss.config.mjs",
      "tailwind.config.ts",
      "tsconfig.json",
    ]) {
      copyFileSync(join(frontendDirectory, name), join(directory, name));
    }
    // Reuse the real config and its plugins. The explicit workspace boundary
    // permits dependency/public symlinks back into frontend, outside this app.
    writeFileSync(
      join(directory, "next.config.js"),
      `import config from ${JSON.stringify(pathToFileURL(join(frontendDirectory, "next.config.js")).href)};\n` +
        `export default { ...config, turbopack: { ...config.turbopack, root: ${JSON.stringify(repositoryDirectory)} }, outputFileTracingRoot: ${JSON.stringify(repositoryDirectory)} };\n`,
    );
    // Turbopack's app discovery needs source files inside the app root. Clone
    // where supported and otherwise copy; never reuse another run's output.
    cpSync(join(frontendDirectory, "src"), join(directory, "src"), {
      recursive: true,
      mode: constants.COPYFILE_FICLONE,
    });
    // Share inputs, never generated files, caches, env files or credentials.
    for (const name of ["public", "node_modules"]) {
      symlinkSync(join(frontendDirectory, name), join(directory, name), "dir");
    }
    return { directory, cleanup };
  } catch (error) {
    cleanup();
    throw error;
  }
}
