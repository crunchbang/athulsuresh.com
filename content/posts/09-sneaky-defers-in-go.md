+++
title = "Sneaky Defers In Go"
author = ["Athul Suresh"]
date = 2021-07-06
draft = false
+++

What do you think the output of the following code would be?

```go
package main

import "fmt"

func main() {
	input := "hello"
	TestDefer(&input)
}

func TestDefer(input *string) {
	defer fmt.Println(*input)
	*input = "world"
	fmt.Println(*input)
}
```

Given how \`defer\`-ed functions are executed just before the parent function exits, I expected the output to be

```nil
world
world
```

But, on execution it actually prints

```nil
world
hello
```

This is because the arguments are evaluated when the defer is encountered, and not when the deferred function is actually called. Effective Go even has a line [specifically about this behavior](https://golang.org/doc/effective_go#defer) (which I discovered later).

This makes sense if you think of \`defer\` as a function that gets executed each time it is encountered. The result of the execution is that it sets up the deferred function to be executed right before the parent function exits.

This whole journey started with a piece of code I was debugging where updates to a piece of data were not being saved to the underlying storage layer.

```go
func doSomething(txn, ....) {
  ...
  defer store.Save(txn)
  ...
  // modify txn here
  ...
  return
}
```

Wrapping it in an anonymous function helped alleviate my problem

```go
func doSomething(txn, ....) {
  ...
  // fugly, but works!
  defer func() {
    store.Save(txn)
  }()
  ...
  // modify txn here
  ...
  return
}
```

That was an evening well spent. I do find that I enjoy these sort of bug adventures which end up correcting some flawed mental model I previously had about a system. They're the most fulfilling.
