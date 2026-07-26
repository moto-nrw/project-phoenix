#!/usr/bin/env node

import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import path from "node:path";
import { storybookProcessEnv } from "../.storybook/mocks/env.js";

const require = createRequire(import.meta.url);
const storybookPackagePath = require.resolve("storybook/package.json");
const storybookBin = path.join(
  path.dirname(storybookPackagePath),
  "dist/bin/dispatcher.js",
);
const forwardedArgs = process.argv.slice(2);
if (forwardedArgs[0] === "--") {
  forwardedArgs.shift();
}

const child = spawn(
  process.execPath,
  [
    "--unhandled-rejections=strict",
    storybookBin,
    "build",
    // Make the default output dir explicit so the build lands in
    // frontend/storybook-static regardless of how the script is invoked.
    // Callers can still override it: a later --output-dir wins.
    "--output-dir",
    "./storybook-static",
    ...forwardedArgs,
  ],
  {
    stdio: "inherit",
    env: {
      ...process.env,
      ...storybookProcessEnv,
      SKIP_ENV_VALIDATION: "true",
    },
  },
);

child.on("error", (error) => {
  console.error(error);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.exit(1);
  }
  process.exit(code ?? 1);
});
