/**
 * Backward-compatible re-exports.
 *
 * The auth config has been split into:
 * - shared.ts (utilities, callbacks, types)
 * - tenant-config.ts (tenant-specific config)
 * - operator-config.ts (operator-specific config)
 *
 * This file re-exports for existing imports that reference config.ts.
 */

export { tenantAuthConfig as authConfig } from "./tenant-config";
export { _testHelpers, _resetRefreshState } from "./shared";
