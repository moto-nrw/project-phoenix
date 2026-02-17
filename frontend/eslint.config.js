import nextVitals from "eslint-config-next/core-web-vitals";
import tseslint from "typescript-eslint";

/** @type {import('typescript-eslint').ConfigArray} */
const eslintConfig = [
  {
    ignores: [".next", "next-env.d.ts", "coverage"],
  },
  ...nextVitals,
  ...tseslint.configs.recommended,
  ...tseslint.configs.recommendedTypeChecked,
  ...tseslint.configs.stylisticTypeChecked,
  {
    files: ["**/*.ts", "**/*.tsx"],
    rules: {
      // React Compiler rules new in eslint-config-next 16 — disabled for the
      // upgrade. Enable incrementally in a follow-up PR.
      "react-hooks/set-state-in-effect": "off",
      "react-hooks/purity": "off",
      "react-hooks/refs": "off",
      "react-hooks/globals": "off",
      "react-hooks/static-components": "off",
      "react-hooks/preserve-manual-memoization": "off",
      "react-hooks/immutability": "off",
      "react-hooks/use-memo": "off",
      "react-hooks/component-hook-factories": "off",
      "react-hooks/error-boundaries": "off",
      "react-hooks/set-state-in-render": "off",
      "react-hooks/config": "off",
      "react-hooks/gating": "off",
      "@typescript-eslint/array-type": "off",
      "@typescript-eslint/consistent-type-definitions": "off",
      "@typescript-eslint/consistent-type-imports": [
        "warn",
        { prefer: "type-imports", fixStyle: "inline-type-imports" },
      ],
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_" },
      ],
      "@typescript-eslint/require-await": "off",
      // Multi-tenancy: prefer useTenantRouter() over raw useRouter() in tenant-scoped code.
      // Operator pages may disable this inline where useRouter is correct.
      "no-restricted-imports": [
        "warn",
        {
          paths: [
            {
              name: "next/navigation",
              importNames: ["useRouter"],
              message:
                "Use useTenantRouter() from ~/lib/tenant-router instead. Raw useRouter() does not prefix routes with the tenant slug.",
            },
          ],
        },
      ],
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: { attributes: false } },
      ],
    },
  },
  {
    linterOptions: {
      reportUnusedDisableDirectives: true,
    },
    languageOptions: {
      parserOptions: {
        projectService: true,
      },
    },
  },
];

export default eslintConfig;
