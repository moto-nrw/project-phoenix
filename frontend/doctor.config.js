export default {
  ignore: {
    // React Doctor marks these broad refactoring and micro-optimization rules
    // as noisy. Keep the scan focused on correctness, security, accessibility,
    // hydration, and concrete performance defects.
    tags: ["test-noise"],
    overrides: [
      {
        // These query parameters select an already-authorized, loaded row.
        // They never prefill a mutation, role assignment, or permission edit.
        files: [
          "src/app/**/database/permissions/page.tsx",
          "src/app/**/database/roles/page.tsx",
        ],
        rules: ["react-doctor/url-prefilled-privileged-action"],
      },
      {
        // These routes proxy to the application backend API. The tenant slug
        // is encoded as one path segment; no static/CDN/object-store resource
        // is selected by tenant input.
        files: [
          "src/app/api/enrollment/[tenantSlug]/submit/route.ts",
          "src/app/api/enrollment/captcha-config/[tenantSlug]/route.ts",
          "src/app/api/enrollment/phases/public/[tenantSlug]/route.ts",
        ],
        rules: ["react-doctor/tenant-static-proxy-risk"],
      },
      {
        // The stored value is a numeric UI countdown expiry. Password-reset
        // and authentication tokens never enter browser storage.
        files: ["src/components/ui/password-reset-modal.tsx"],
        rules: ["react-doctor/auth-token-in-web-storage"],
      },
      {
        // This layout effect is an asynchronous ownership boundary: it masks
        // the previous child's data and invalidates stale writes before paint.
        files: ["src/components/parent/child-care.tsx"],
        rules: ["react-doctor/no-derived-state-effect"],
      },
      {
        // The effect closes the current EventSource, clears its reconnect
        // timer, and removes both global listeners in its cleanup function.
        files: ["src/lib/hooks/use-sse.ts"],
        rules: ["react-doctor/effect-needs-cleanup"],
      },
    ],
  },
  rules: {
    // Context providers and UI modules intentionally colocate hooks, types,
    // and small helpers. This only affects development Fast Refresh state; it
    // has no production runtime impact and splitting these APIs would make the
    // modules harder to use.
    "react-doctor/only-export-components": "off",

    // Small private subcomponents are intentionally kept next to their sole
    // consumer. File count is not a useful maintainability gate by itself.
    "react-doctor/no-multi-comp": "off",

    // Next.js loads sharp implicitly for production image optimization. There
    // is no direct import for a dead-code analyzer to discover.
    "deslop/unused-dependency": "off",
  },
};
