# Internals

This guide gives an overview of the internals of the Glide compiler and execution engine.

In general, it's worth reading through the source code and playing with the tests in this repo if you are working on the compiler and execution engine. We use table-driven tests and have fairly good coverage over the parsing, compiling and execution methods.

## Parsing

```
YAML definition -> Program
```

To run a Glide workflow, we start by unmarshalling the YAML workflow definition file into a `glide.Program{}` struct.

A `Program` struct is a map of several `glide.Path` structs, which themselves contain a slice of `step.Step` structs.

To do this, we call:

```go
program, err := glide.Unmarshal(yamlBytes, glide.Dialect{}) // replace with the glide dialect you're using
```

Internally, this calls `yaml.Unmarshal` on the data. We have implemented custom `UnmarshalYAML` methods on the `Program`, `Path` and `Step` structs to parse the input data.

## Compiling

```
Program -> Graph
```

The next step in executing a Glide workflow is compiling the Program. To do this, we construct a `glide.Compiler`:

```go
var schema jsoncel.Schema
err = json.Unmarshal(schemaBytes, &schema) // read schemaBytes from a file, etc
if err != nil {
  return err
}


compiler := glide.Compiler{
  Program:     prog,
  InputSchema: &schema,
}
```

The compiler needs to know the schema of the input. This is because we type-check all CEL expressions during compilation, to determine whether any expressions reference invalid variables.

To compile the graph, we call `compiler.Compile()`:

```go
g, err := compiler.Compile()
if err != nil {
  return err
}
```

The compile method visits each statement in the program. Each time it visits a statement, it adds a new node to the Execution Graph. It creates edges in the Execution Graph based on the ordering of the statements. You can read the implementation in [`compile.go`](/compile.go).

To illustrate how compilation works we can take a simple workflow:

```yaml
workflow:
  default:
    - start: A
    - outcome: B
```

We start at the first statement in the first path:

```yaml
- start: A
```

We read the statement and create a corresponding node in our graph:

```
GRAPH

[start - A]

```

We then read the next statement:

```yaml
- outcome: B
```

We create a corresponding node for B in our graph:

```
GRAPH

[start - A]      [outcome - B]
```

We then link the previous step with the step we've just read:

```
GRAPH

[start - A]  ->  [outcome - B]
```

By iteratively repeating this process we build up the Execution Graph.

Internally the graph is represented as a directed acyclic graph (DAG), using the `github.com/dominikbraun/graph` graph package.

## Execution

```
Graph -> Results
```

To execute the graph we call the following method:

```go
results, err := g.Execute("request", input)
```

Where `"request"` is an example of a start node to begin execution from, and `input` is the input data to execute the workflow with.

To execute the workflow we perform a breadth-first search on the graph, starting at the start node. For each node, we check whether the node is complete, and whether it's predecessors are complete. You can read the implementation in [`execute.go`](/execute.go).

## Error handling

Errors during parsing and compiling are wrapped in a `noderr.NodeError`. This error struct contains information about the YAML node which caused the error, and can be used to display a lint error to the user who wrote the Glide workflow:

```go
p, err := glide.Unmarshal(data, cf.Dialect)

var ne noderr.NodeError
if errors.As(err, &ne) {
  clio.Infof("node error at: %s", ne.Node.GetPath())
  source, printErr := ne.PrettyPrint(data)
  if printErr != nil {
    clio.Errorf("error pretty printing YAML path: %s", printErr)
  }
  fmt.Fprintf(os.Stderr, "%s\n", source)
}
```
