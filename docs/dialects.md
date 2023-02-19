# Dialects

The available action types, start, and outcome nodes are provided by a Glide dialect. The examples above use the Common Fate Glide dialect, which has been designed for access request workflows.

It is possible to develop a separate dialect to introduce different actions into a workflow. Here's another dialect designed for customer support workflows:

```yaml
workflow:
  customer_support:
    steps:
      - start: ticket_submitted

      - action: triage
        with:
          owner: chris

      - check: ticket.priority == "P1"

      - action: open_incident
        with:
          owners: [devs]

      - check: ticket.is_resolved

      - outcome: resolved
```

A minimum Glide dialect is defined as follows, using Go:

```go
var Dialect = dialect.Dialect{
  // the available Actions
	Actions: func() map[string]any {
    return map[string]any{
      "approval": &Approval{},
    }
  },
  // the available start/outcome nodes
	Nodes: map[string]node.Node{
		"request":  {Type: node.Start, Name: "Request"},
		"approved": {Type: node.Outcome, Priority: 1, Name: "Approved"},
	},
}

type Approval struct {
	Groups []string `yaml:"groups"`
}
```

You can look at the Common Fate dialect in [cf.go](/pkg/dialect/cf/cf.go) to get a feel for how a dialect is implemented.

When Glide parses a workflow definition, it calls `yaml.Unmarshal()` on the Action struct (`Approval` in the above example).

You can add `yaml` tags to the struct fields as shown above to allow the `with` section of an action definition to be parsed. For example:

```go
var Dialect = dialect.Dialect{
	Actions: func() map[string]any {
    return map[string]any{
      "do_something": &Something{},
    }
  },
}

type Something struct {
	Foo string `yaml:"foo"`
}
```

If the YAML definition looks like this:

```yaml
workflow:
  example:
    steps:
      # ...
      - action: do_something
        with:
          foo: bar
```

The `Something` action will be constructed with `Foo = bar`, e.g.

```go
Something{Foo: "bar"}
```

[Back to README](/README.md)
