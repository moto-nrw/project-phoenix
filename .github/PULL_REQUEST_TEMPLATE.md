# Pull Request

## Description
<!-- Explain the changes you've made and why they're needed -->

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update
- [ ] Configuration change
- [ ] Security improvement
- [ ] Refactoring
- [ ] Performance improvement
- [ ] Test addition/modification
- [ ] CI/CD improvement

## Related Issues
<!-- List any related issues using the following syntax:
- Fixes #123
- Addresses #456
- Related to #789
-->

## Testing
<!-- Describe how you tested your changes -->
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed
- [ ] Testing documentation updated

## Backend Architecture Migration (#2580)
<!-- TEMPORARY until #2580 closes. Required for every change under backend/.
     Follow backend/CLAUDE.md § "Active Architecture Migration (#2580)".
     For non-backend PRs, write "Not applicable — no backend changes". -->
- Target owner/capability:
- Touched shrink-only surfaces and before → after counts:
- [ ] `scripts/backend-architecture.sh check` passed and I inspected all policy, baseline, and composition-inventory changes

## Security Checklist
<!-- This section is mandatory for all PRs -->
- [ ] I have followed the [security guidelines](../docs/security.md)
- [ ] No sensitive information is committed (secrets, credentials, certificates)
- [ ] Environment variables are properly managed with templates
- [ ] Code does not contain hardcoded secrets
- [ ] No potential security vulnerabilities introduced

## Screenshots or Examples
<!-- Add screenshots or code examples if applicable -->

## Missverständnis-Check
<!-- Mandatory for user-visible changes (tenant portal, parents portal, kiosk, e-mails, help guide).
     Checklist: .claude/rules/verstaendlichkeit.md — name the blocks you checked and what you changed.
     Write "nicht user-sichtbar" if it does not apply. -->

## Checklist
- [ ] My code follows the project's style guidelines
- [ ] I have performed a self-review of my code
- [ ] I have commented my code, especially in hard-to-understand areas
- [ ] I have updated the documentation as needed
- [ ] If this PR changes a user-facing flow, I updated the in-app help guide (`guide-data.ts`) and any changed screenshot — or it doesn't apply (backend-only, operator/parents-portal-only, or pure-styling change)
- [ ] User-visible change: I ran the Verständlichkeit checklist (`.claude/rules/verstaendlichkeit.md`) against the running app and recorded the result above — or it doesn't apply
- [ ] All tests are passing
- [ ] My changes generate no new warnings or errors
- [ ] I have checked for and resolved any merge conflicts

## Additional Notes
<!-- Add any other context about the PR here -->
