# glide

Read these first:

- [Overview](./docs/overview.md)
- [Dialects](./docs/dialects.md)
- [Internals](./docs/internals.md)

## Getting started

We recommend installing GraphViz:

```
brew install graphviz
```

Run an example (piping the output to GraphViz for visualisation):

```
go run cmd/main.go run -f examples/basic/workflow.yml -s examples/basic/schema.json -i examples/basic/input.json | dot -Tpng > example.png
```
