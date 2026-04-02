# Community Naming Conventions

## The `ext.ocm.software` Namespace

The prefix `ext.ocm.software` is the **single reserved namespace** for any extension
or metadata produced by community projects in this repository.

### Rationale

- Mirrors Kubernetes-style annotation/label namespacing
- Clearly distinguishes community extensions from OCM core (`ocm.software`) and
  specification-defined keys
- Applies uniformly across all metadata surfaces

### Scope

The `ext.ocm.software` prefix applies to:

**OCM component descriptor labels and annotations**

Any label or annotation key on an OCM component version, resource, source, or reference
produced by a community project MUST use `ext.ocm.software/<key>`:

```yaml
labels:
  - name: ext.ocm.software/my-plugin.feature-flag
    value: "enabled"
```

**Kubernetes resource annotations and labels**

Any Kubernetes annotation or label introduced by a community project MUST use
`ext.ocm.software/<key>`:

```yaml
metadata:
  annotations:
    ext.ocm.software/project: my-oci-plugin
```

**Any other metadata**

CI outputs, plugin identifiers, config keys, and any other metadata originating from
a community project SHOULD use `ext.ocm.software` to distinguish it clearly from
core OCM tooling.

### What this namespace is NOT

- Not used by the OCM specification or core library — those use `ocm.software`
- Not used by SIG Runtime's core deliverables (CLI, controller, bindings)

## Directory Naming

Project directories under `projects/` MUST use kebab-case with no namespace prefix:

```
projects/
  my-oci-plugin/
  flux-integration/
  helm-component-constructor/
```