---
paths:
  - "backend/**/*.go"
  - "frontend/src/**/*.ts"
  - "frontend/src/**/*.tsx"
  - "docs/adr/**/*.md"
---

# Information flow, not data silos

Apply the [product principle](../../CLAUDE.md#product-principles) when designing,
implementing or reviewing data collection, access rules and cross-module or
cross-portal flows. A module owns its data and invariants, not exclusive use of
the information by that module's users.

## Decision and review

1. **Name the task and recipients.** Identify who needs which information,
   for what purpose and for how long. Follow the whole flow from collection to
   use, including downstream lists, profiles and exports where relevant.
2. **Make the normal flow complete.** When purpose, lawful basis and recipient
   authorization are established, provide the needed information at the point
   of use by default. Avoid manual re-entry, extra sharing switches or repeated
   releases unless they serve a concrete domain or legal requirement.
3. **Justify restrictions.** Name the domain or legal reason for withholding
   needed information. A portal, module or storage boundary alone is not a
   reason. If the lawful basis or recipient scope is unresolved, identify that
   exact gap; neither assume permission nor turn uncertainty into a permanent
   product restriction. Surface conflicts with existing ADRs and record the
   replacement decision rather than silently overriding them.
4. **Preserve one authoritative source.** Reuse information through the owning
   module's supported contracts. Keep necessary projections or temporary copies
   synchronized with corrections, cancellations and expiry. Distinguish
   purpose-specific information from existing master data instead of silently
   overwriting it.

## Access boundaries and proof

Sharing means access for the people performing the task, not visibility for
every account in the same organization. Preserve relationship-level permissions,
tenant and portal isolation, audit requirements, retention and revocation.
Cross-school access needs an explicit authorized relationship, not a broader
session or an unscoped query; follow [the contracts](../../docs/agents/contracts.md).

For health and other sensitive data, establish the applicable lawful basis and
any necessary consent for the actual purpose and recipients. A privacy notice
alone does not establish that basis. New documentation or a sharing default
does not retroactively authorize access to existing records.

Before completing a behavior change, verify that an authorized recipient gets
the needed information in the normal workflow, an unrelated recipient cannot
read it, and correction, withdrawal or expiry ends or updates access as required.
Document any remaining gap explicitly; static documentation checks alone do not
prove runtime information flow.
