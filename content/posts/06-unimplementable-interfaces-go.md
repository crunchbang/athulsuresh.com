+++
title = "Un-implementable Interfaces In Go"
author = ["Athul Suresh"]
date = 2021-05-07
draft = false
+++

Recently, I started randomly going through the Go standard library, mostly to satiate my curiosity and to find out what goes on behind the curtains. While checking out the testing package, I found this interesting little snippet of code in `src/testing/testing.go`:

```go
// TB is the interface common to T and B.
type TB interface {
	Cleanup(func())
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fail()
	FailNow()
	Failed() bool
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Helper()
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Name() string
	Skip(args ...interface{})
	SkipNow()
	Skipf(format string, args ...interface{})
	Skipped() bool
	TempDir() string

	// A private method to prevent users implementing the
	// interface and so future additions to it will not
	// violate Go 1 compatibility.
	private()
}
```

This seems pretty evident once you see it. It makes sense for the Go standard library where the private function enables them to circumvent the compatibility promise by ensuring that no one would be able to use this interface outside of the standard library because of the private function. This gives them the flexibility to add functionality later without breaking anything.

I wonder if there's a general lib out in the wild which uses this technique.
