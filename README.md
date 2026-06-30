# lord-helmchen

```sh
# Converts a values.yaml file into a CRD schema and applies it to the cluster.
go run ./schemagen ../vshnjuiceshop/values.yaml | ka apply -f-
```
