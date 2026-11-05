# Security policy

## Reporting a vulnerability

Do not report a security vulnerability in a public issue, a pull request or a discussion.

Report it privately through GitHub: open the **Security** tab of this repository and choose
**Report a vulnerability**. Include what you found, how to reproduce it, and the version or commit
you tested.

Please give us time to release a fix before you disclose the vulnerability.

## Scope

The learner and admin apps (`apps/app`, `apps/admin`, `packages/*`), the API (`apps/api`) and the
container images. The Helm chart lives in its own repository,
[`genesary/pupitre-helm`](https://github.com/genesary/pupitre-helm): report its vulnerabilities
there.

The test accounts and passwords of the local Keycloak realm
(`apps/api/infra/keycloak/pupitre-realm.json`) are public on purpose and are not a vulnerability.
