# Expression Language   (CEXL)

Istio (mixer) configuration uses an expression language to evaluate selectors and input maps.
The syntax of the expression language is C-like.

An expression maps typed Attribute Values to a Value. 

```
f(attrValue0, attrValue1, ..., constant0, constat1, ...) --> Value
```

## Predicates
Selector expressions are predicates that map attribute values to booleans.
```
f(attrValue0, attrValue1, ..., constant0, constat1, ...) --> Boolean
```
### Examples of selector expressions

1. select all traffic that is destined for service1 and source user is admin.
`target.service == "service1.ns1.cluster" && source.user == "admin"`
2. select all requests that did not result in a error.
`response.status_code < 400`
3. select requests which were initially send to host "aaa".
`request.header["X-FORWARDED-HOST"] == "aaa"`
4. select all traffic that is destined for namespace ns1 and source user is admin.
`target.service == "*.ns1.cluster" && source.user == "admin"`

## Mapping expressions
Mapping expressions map attribute values to a typed value.

### Examples of mapping expressions

1. evaluates to request.size if available, otherwise 200
`request.size| 200  --> int`
2. evaluated to request.user if available, otherwise "nobody"
`request.user | "nobody" --> string`

Expression may use any valid attributes

## Grammar
Expression language uses a subset of C like expressions.
The front end uses golang lexer and parser. The AST produced by the parser is converted to a simpler AST that only consists of Functions, Variables and Constants. Constants are implcitly typed. Variable names are treated as attribute name and therefore inherit type from the attribute definition. Function return values are either statically typed like `EQ(a, b ) --> Boolean` or dynamically typed like `OR(T a, T b) --> T`.

Given a vocabulary every expression is validated and resolved to a type.
Selectors __must__ resolve to a boolean value. Mapping expression should resolve to the type they are mapping into.   
## Functions
Expression language is extended by implementing the [Func interface](https://github.com/istio/mixer/blob/master/pkg/expr/func.go#L25). It is a small but growing list of functions. 

Typechecker ensures that all functions are called with the correct number of arguments of the correct type. The effective type of an expression is inferred by processing its AST. 
