// Turbopack-Bundle-Analyse mit vollständiger Server-Env (pnpm run perf:bundle,
// #2938). `next experimental-analyze --output` schreibt nach
// .next/diagnostics/analyze; proxy.ts verlangt die Hostnames schon beim Build.
import { spawnSync } from "node:child_process";

import { perfServerEnv } from "./env.mjs";

const result = spawnSync(
  "pnpm",
  ["exec", "next", "experimental-analyze", "--output"],
  {
    stdio: "inherit",
    env: { ...process.env, ...perfServerEnv(), SKIP_ENV_VALIDATION: "true" },
  },
);
process.exit(result.status ?? 1);
