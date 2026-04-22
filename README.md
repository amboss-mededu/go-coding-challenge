# Schema-Driven Code Generator Challenge

## Goal

Build a code generator that reads JSON Schema files and produces Go type definitions.

## Setup

Requires Go 1.23+.

- `schemas/` contains JSON Schema files describing three types: Product, Category, and Article.
- `testdata/` contains JSON payloads that conform to these schemas.
- `model/` is where your generated Go code should go. It ships with stub types so that the project compiles.
- `harness_test.go` imports the types from `model/` and unmarshals each payload into the corresponding type.

## Your Task

Write a code generator that reads the schemas in `schemas/` and produces Go types in `model/`. The generator can be a script, a CLI or just a library, it's up to you. What matters is that the output _faithfully_ represents the schema. This means:

- Each named type from the JSON schema should have a Go equivalent
- Each field in the JSON schema should have a Go equivalent (not necessarily 1-to-1)
- Whether or not a field is optional should be encoded in the Go types in some manner
- We also need a Go representation of concepts such as `oneOf`

The test harness (`go test`) is a basic smoke test which unmarshals payloads into your types and checks for errors. It's possible to make all tests pass even though the generated Go code is still missing a lot of necessary information.

## Rules

- Use any tools you want, including LLMs.
- The generated types must live in the `model` package.
- The type names must match the schema titles: `Product`, `Category`, `Article`.
- You may add any additional types as needed (sub-structs, enums, unions, etc.).

## What to deliver

- A private fork or copy of this repository with your solution
- A README covering: how to run the generator, what decisions you made and why
- Brief notes for a walkthrough — how you approached it, what trade-offs you made

## Time

Aim for about 3 hours. It's fine to leave things out — just tell us what you'd add with more time.
