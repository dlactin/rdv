# Usage

This must be run while your current directory is within your git repository.

## Flags

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--debug` | `—` | Enable verbose logging for debugging | `false` |
| `--github` | `g` | Output format optimized for GitHub PR comments | `false` |
| `--help` | `h` | help for rdv | `false` |
| `--no-color` | `—` | Output in plain style without any highlighting | `false` |
| `--output` | `o` | Write the local and target rendered manifests to a specific file path | `—` |
| `--path` | `p` | Relative path to the chart or kustomization directory | `.` |
| `--ref` | `r` | Target Git ref to compare against. Will try to find its remote-tracking branch (e.g., origin/main) | `main` |
| `--semantic` | `s` | Enable semantic diffing of k8s manifests (using dyff) | `false` |
| `--update` | `u` | Update Helm chart dependencies. Required if lockfile does not match dependencies | `false` |
| `--validate` | `v` | Validate rendered manifests with kubeconform | `false` |
| `--values` | `f` | Path to an additional values file (can be specified multiple times) | `[]` |
| `--version` | `—` | version for rdv | `false` |

## Examples

### Checking a Helm Chart diff against another target ref

```bash
rdv -p ./examples/helm/helloworld -f values-dev.yaml -r development
```

### Checking a Helm Chart diff and validating our rendered manifests

```bash
rdv -p ./examples/helm/helloworld --validate
```

### Checking Kustomize diff against the default (main) branch with semantic diff

```bash
rdv -p ./examples/kustomize/helloworld -s
```

### Checking Kustomize diff against a tag

```bash
rdv -p ./examples/kustomize/helloworld -r tags/v0.5.1
```
