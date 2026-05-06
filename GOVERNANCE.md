# Governance

## Relationship to OCM Project Governance

This repository operates under the OCM project's governance model as defined by:

- [OCM Technical Charter](https://github.com/open-component-model/open-component-model/blob/main/docs/steering/CHARTER.md)
- [SIG Handbook](https://github.com/open-component-model/open-component-model/blob/main/docs/community/SIGs/SIG-Handbook.md)

## Governance Chain

**TSC → Project Maintainers**

The OCM Technical Steering Committee (TSC) owns and governs this repository directly
until the community grows sufficiently to warrant a dedicated SIG or Working Group.
The TSC approves all project submissions and retains authority over the repo.

TSC contact: `open-component-model-tsc@lists.neonephos.org`

## Project Admission

All new projects require TSC approval before merge:

- A majority vote of the TSC MUST approve any new project submission
- Approval is enforced natively via CODEOWNERS — a TSC member must approve the PR

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full submission process.

## Project Lifecycle

- **Incubating**: recently merged, in evaluation period
- **Active**: actively maintained by project owner(s)
- **Archived**: no longer maintained; archived by TSC consensus
- **Removed**: removed by 2/3 supermajority of TSC Voting Members

The TSC conducts periodic health checks on active projects. Projects inactive for
6+ months are considered for archiving (consistent with SIG lifecycle rules in the SIG Handbook).

## Code Ownership

Project submissions must include a CODEOWNERS entry assigning maintainers to the
project directory. The TSC reviews and approves these entries as part of the
submission PR. Project owners have full autonomy over their top-level project
directory but cannot modify repository-wide CI workflows. Changes to CODEOWNERS
remain TSC-controlled to prevent unilateral privilege escalation.

## Governance Evolution

When this repository grows sufficiently — measured by number of active projects,
contributor diversity, and sustained activity — the TSC may elect to charter a
dedicated SIG or Working Group to take over day-to-day governance. This transition
requires a TSC majority vote and an update to this document.

## Amendments

Changes to this file require TSC approval (enforced via CODEOWNERS).

## Code of Conduct

All participants must comply with the
[NeoNephos Code of Conduct](https://github.com/neonephos/.github/blob/main/CODE_OF_CONDUCT.md).

## Licensing

All contributions must use Apache-2.0 or MIT, consistent with the
[OCM Technical Charter §7](https://github.com/open-component-model/open-component-model/blob/main/docs/steering/CHARTER.md).