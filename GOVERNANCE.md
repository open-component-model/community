# Governance

## Relationship to OCM Project Governance

This repository operates under the OCM project's governance model as defined by:

- [OCM Technical Charter](https://github.com/open-component-model/open-component-model/blob/main/docs/steering/CHARTER.md)
- [SIG Handbook](https://github.com/open-component-model/open-component-model/blob/main/docs/community/SIGs/SIG-Handbook.md)

## Governance Chain

**TSC → SIG Runtime → Project Maintainers**

The OCM Technical Steering Committee (TSC) charters this repository and retains
ultimate authority. Day-to-day oversight is delegated to **SIG Runtime**.

| Role          | Name              | GitHub       |
|---------------|-------------------|--------------|
| SIG Chair     | Gergely Bräutigam | @skarlso     |
| SIG Tech Lead | Fabian Burth      | @fabianburth |

TSC contact: `open-component-model-tsc@lists.neonephos.org`

## Project Admission

All new projects under `projects/` require SIG Runtime approval before merge:

- At least one of SIG Runtime Chair or Tech Lead MUST approve any new project submission
- Routine submissions may be approved by any single SIG Runtime Voting Member
- The `ext.ocm.software/approved` label on the PR signals approval; the merge gate enforces it

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full submission process.

## Project Lifecycle

- **Incubating**: recently merged, in evaluation period
- **Active**: actively maintained by project owner(s)
- **Archived**: no longer maintained; archived by SIG Runtime consensus + TSC notification
- **Removed**: removed by 2/3 supermajority of SIG Runtime Voting Members + TSC notification

SIG Runtime conducts quarterly health checks on active projects. Projects inactive for
6+ months are considered for archiving (consistent with SIG lifecycle rules in the SIG Handbook).

## Code Ownership

Project-specific CODEOWNERS entries are added by SIG Runtime after a project merges.
Project owners have full autonomy over their `projects/<name>/` directory but cannot
modify repository-wide CI workflows. The `OWNERS.md` file within each project remains
SIG Runtime-controlled to prevent unilateral privilege escalation.

## Amendments

Changes to this file require approval from both the TSC Chair and the SIG Runtime Chair
or Tech Lead (enforced via CODEOWNERS).

## Code of Conduct

All participants must comply with the
[NeoNephos Code of Conduct](https://github.com/neonephos/.github/blob/main/CODE_OF_CONDUCT.md).

## Licensing

All contributions must use Apache-2.0 or MIT, consistent with the
[OCM Technical Charter §7](https://github.com/open-component-model/open-component-model/blob/main/docs/steering/CHARTER.md).