# Kustomize files

## Sample usage

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: glob

resources:
  - https://github.com/baarsgaard/glob/deploy/
```

Full config under [full_sample](./full_sample)
