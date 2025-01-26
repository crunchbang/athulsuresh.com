+++
title = "Notes On Erlang"
author = ["Athul Suresh"]
date = 2022-11-23
tags = ["erlang"]
draft = false
+++

Collection of unstructed notes that I accumulated while learning Erlang. This is a _work-in-progress_.

-   erlang programs are composed on communicating processing. Like modeling objects, processes should be modeled to fit the problem. This is called modeling concurrency.
-   concurrency is about structure. parallelism is about execution.
-   each expression must end with a `.`
-   variables in erlang can only be bound once. Variables start with UPPERCASE letters. atoms begin with lowercase letters.
-   processes evaluate fns that are defined in modules. Modules are files with `.erl` extension.
-   `pid ! {client_pid, message}` for sending message to pid.
-   `c(module_name)` to compile the module. `f()`  to forget the bindings.
-   file server and client. client provides the abstraction and hides the details of communication with the actual process. This gives use the flexibility to change the underlying implementation without changing the interface exposed through the client. _It's refreshing how Joe Armstrong doesn't talk down to the programmer in this book. The innards and complicated lingua franca is exposed for everyone to see._
-   erlang can handle arbitrary precision numbers. Like really big numbers.
-   = is more of a pattern matching operator rather than assignment operator.
-   atoms are similar to symbolic constants or enums. atoms are global. atoms can be quoted and can have spaces in them.
-   `{item1, blah}` represents a tuple of fixed size. Since tuples don't have type, it's a convention to add an atom as the first element indicating the type. `{point, 0, 1}`. `{point, C, C} = {point, 25, 25}` works!
-   strings are represented as a list of integers with each int representing an unicode point.
-   fullstop separates expression. comma separates related subordinate clauses. semicolon separates clauses.
-   `fun(arg) -> body end.` to define anonymous fns.
-   `[f(X) || X <- L]` list comprehension. `X <- L` follows the pattern logic for =.
-   Named function F/n should be passed in as `fun F/n` when used as an argument
-   `-record(Name { field1 = DefaultValue1, .... fieldN }`.  `undefined` is the default value. `.hrl` files are like C header files where common definitions can be kept. `#Name{key1=val1...}.` to instantiate the struct. `rr("record_file.hrl")` to bring it into the erl.
-   `X#Name{field1=NewValue}` to create a new record from an existing record X with a field value changed.
-   Maps are weird. `#{ key Op val, key2 Op val2}`. `:=` for updating an existing key. =&gt; for adding a new key. The update follows the same pattern as records.
-   `;` is OR and `,` is AND for guard sequences. maps : get/find etc for acessing map values.
-   clauses of a fn need to be separated by a SEMICOLON instead of a PERIOD!
-   `<<"binaries">>`. Binary values must be in the range 0-255. Any other value would wrap around and be mapped to a value in the 0-255 range. term_to_binary and binary_to_term.
-   a type is binary when it's size is divisible by 8, otherwise it's a bitstring.
-   `<<R:5, B:6, G:5>>` to pack elements into a binary while specifying the bit size. The same pattern HAS to be used when unpacking.
-   type test BIFs are allowed in guard clauses. `is_xxx`
-   `self()` to get the PID of the current process. Sending the current process's PID in the message is a convention that allows the receiver to know whom to reply to.
-   Storing state of the function on the stack i.e on function parameters
-   Modules have functions and attributes. attrs start with `-`
-   `-define(MACRO, val)` would be used as `?MACRO`
-   `receive..after..end` to specify timeouts. Sounds similar to Go timer ticks
-   `spawn(Module, Fn, Arg)` to spawn a new process that executes Fn. Args is a list of args that will be passed to Fn. Arg will **always** be a list.
-   with a full mail box, the messages are tried in order. If a msg doesn't match any of the patterns, then it is put on the save q and the next message is tried. If it matches, then the messages from the save q is put back on top of the mailbox. This is called selective receive. nest another receive in a fn within the `after 0` block to do selective receive.
-   messages that do not match a pattern are never lost. They're always around. The downside is this could lead to mailbox pollution if the proc doesn't have the patterns to receive the msg.
-   In a defensive programming style, the `Unexpected` match is used as a catchall to prevent mailbox pollution.
-   `link(pid)` links the current process with pid. `unlink(pid)`. When one of the linked processes crashes, the other crashes (exits?) too. Links are bidirectional.
-   Since link(spawn(..)) is a multistep op, there could be a case where the process dies before it is spawned. This can cause undefined behaviour. In order to avoid this spawn_link(..) can be used which works in an atomic way.
-   `exit(blah)` gets propagated as a special message ("signal") which cannot be caught using normal receive. It can be caught if `process_flag(trap_exit, true)` is set. Then `{'EXIT', Pid, msg}` can be caught.
-   monitors are like links, but they're unidirectional and can be stacked. `erlang:monitor(process, spawn(...))`. spawn_monitor is the atomic alternative.
-   `exit(Pid, reason)` to kill another process.
-   Each process can be registered against a name which can then be used for sending messages instead of Pid. `register(name, pid)`. `unregister(name)`. `registered() / regs()` to get info about the registered processes. A process can have only name and a name can only be registered once. `whereis(registered_name)` to get the pid associated with it. This can be used in patterns to ensure that the reply is indeed from the process we expect.
-   Another pattern is to send a ref to the proc and expect it back in the reply. This prevents us from expecting a reply from a specific Pid and shields us against scenarios where the process gets restarted. `make_ref()`. Refs are used when we expect a message from a certain source. In cases, we expect a message (ex: notification) and don't care about the source, then the ref can be omitted.
-   If a monitor is set for a process that's already down using `erlang:monitor(process, PID)` then we receive a `{'DOWN', MonitorRef, process, PID, <reason>}` message as the reply.
-   A pattern that I've seen is that the proc file contains method that can be used for invoking all the messages that the proc expects.
-   Calls that require reply follow the pattern `{Pid, Ref, Msg}`

> "Walking on water and developing software from a specification are easy if both are frozen."
