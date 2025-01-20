+++
title = "Notes from 'Linux Kernel Development'"
author = ["Athul Suresh"]
date = 2020-06-16
draft = false
+++

This book had been on my TO-READ list for a long time. It came up again while I was perusing [Dan Luu's Programming book list](https://danluu.com/programming-books/). I've always wanted to look behind the curtains and see how the magic worked, so I finally bought it.

I used [bootlin](https://elixir.bootlin.com/linux/v5.7.2/C/ident/task_struct) to read through Linux 5.7.2 source. They provide a really good search system and linked definitions. The book describes kernel version 2.6. You might want to keep this site open to see how things have changed since then.


## Process &amp; Threads {#process-and-threads}

A process begins it life with `fork()`

Lifecycle: `fork()` [Create a copy of the current running process] -&gt; `exec()` [Load a binary into memory] -&gt; `exit()`

Metadata about each process is stored in a `task_struct`. Info about all processes are maintained in a linked-list called the tasklist. They're often referred to as process descriptors.

`thread_info` struct is present at the bottom of the stack (for stacks that grow down). This allows for a lot of neat optimizations whereby the `thread_info` of the current process can be computed and found pretty quickly(review).

`fork()` is implemented through Copy-On-Write (COW) pages. Resources are duplicated only when they are modified. The gain comes through not duplicating the address space!

Threads in linux are no different from processes. Each thread has it's own `task_struct` and is scheduled like any other task. Certain params in the `task_struct` have common values to indicate that resources are shared. This is different from Windows where threads are seen as lightweight processes, where the kernel has explicit support for dealing with threads.

Kernel threads are a special class of threads that run only in kernel space. Forked from `kthreadd` for performing special ops like flush, ksoftirqd.


## Scheduling {#scheduling}

O(1) scheduler, followed by the Completely Fair Scheduler

Sticking to conventional ideas of an absolute time slice ensures constant switching rate but variable fairness and can lead to a slew of problems. CFS does away with this by ditching timeslices and allocating a portion of the processor to each process. This results in variable switching rate but constant fairness.

CFS works by assuming that there is an ideal processor that is capable of multitasking. If we have n processes, each would run in parallel, consuming 1/n of the processor. Reality deviates from this ideal dream in the fact that perfect multitasking is not possible, and that there is an overhead involved in switching processes. Nevertheless, CFS is designed with the idea of giving a portion of the processor to each running process. This portion assigned is a function of the total number of processes waiting to be run. Nice values are used here to weight the processor portion that each process receives - a lower niceness value would result in a relatively higher portion of the processor. Thus when we take an infinitely small time window, each process would've been scheduled for a time slice proportional to their processor portion.

This infinitely small window is usually approximated to a duration called `targeted latency`. Smaller value results in higher interactivity since it approximates the ideal case, but it results in lower throughput because of switching overhead. `targeted latency` is floored at a value called `minimum granularity` by the kernel.

All the scheduling info is carried in `sched_entity` which is embedded in each `task_struct`.

The most interesting thing here is the `vruntime`, the virtual run time, which is what the scheduler uses to pick the next process. There is a concept of physical time and virtual time. Physical run time is the actual time that the process ran and virtual run time is normalized physical time computed using the number of runnable processes and the niceness value of the process. Approximately, it is computed as `physcial_time * (NICE_0_LOAD / proc_load)` where `NICE_0_LOAD` represents the weight of a process who's niceness value is 0 and `proc_load` represents the weight of the process calculated using its niceness value. Thus for processes with lower niceness value (higher priority), the virtual time would be less than physical time and vice versa. Thus they'd get a bigger portion of the processor in turn. This [SO](<https://stackoverflow.com/questions/19181834/what-is-the-concept-of-vruntime-in-cfs/19193619>) answer goes into some more depth.

CFS maintains runnable procs in a red-black tree where the key is the `vruntime`. It continuously picks and schedules the process with the lowest `vruntime`. It does a neat optimization where it caches the left-most node during insertion / deletion of each new node.

When a task goes to sleep, it marks itself as sleeping, puts itself on a wait Q, removes itself from the red-black tree of runnables and calls `schedule()` to select the new process to execute. To wake up the task, it is marked as runnable, removed from the wait Q, and put back in the runnable tree.


## System Calls {#system-calls}

System calls provide an interface between the applications in user space and the kernel. They provide a mechanism through which applications can safely interact with the underlying hardware, create new processes, communicate with each other, as well as the capability to request other operating system resources. Provide mechanism, not policy. The kernel system calls provide a specific fn. The manner in which it is used does not matter to the kernel.

User space applications cannot directly invoke a kernel function. The whole communications happens through register values and interrupts. Each syscall has a particular value associated with it. This value is loaded into the `eax` register and then an interrupt is invoked `int 0x80` which invokes the interrupt handler which hands over control to the kernel, which then executes the appropriate system call on behalf of the user space application.

Most of the system calls are defined with the funky `SYSCALL_DEFINE` macro. This [answer](<https://www.quora.com/Linux-Kernel/Linux-Kernel-What-does-asmlinkage-mean-in-the-definition-of-system-calls>) explains the curious `asmlinkage` that gets prefixed to these functions. Syscall `bar` is referred to as `syscall_bar` within the kernel.


## Kernel Data Structures {#kernel-data-structures}

The ubiquitous linked list implementation is a circular doubly linked list... with some quirks. Unlike usual linked lists, the data is not embedded within the linked list struct but rather the linked list struct `struct list_head` is embedded within the data struct. The kernel uses some C macro magic with `container_of` to get a pointer to the embedding struct from the `list_head` pointer. This [post](<https://radek.io/2012/11/10/magical-container_of-macro/>) demystifies the magic behind the macro.

In addition the kernel code also contains implementations for a queue (with the usual ops) and a map. The map is implemented as a balanced binary search tree with a rather confusing name - idr. It provides mapping between UIDs to pointers.


## Interrupts &amp; Interrupt Handlers {#interrupts-and-interrupt-handlers}

Interrupts generated by H/W are handled by specific Interrupt Handlers or Interrupt Service Routines (ISR). Generally the ISR for a device is part of the device driver code in the kernel. ISR in the kernel are nothing but C functions that run in the interrupt context (atomic context). The work associated with handling an interrupt is divided into two parts -

1.  Acknowledging the H/W and performing operations that will enable the H/W will proceed further (stuff like copying all the received packets from a NIC's buffer) - This is handled by the 'Top Half'.
2.  Further work on the data associated with the kernel, which is not critical and can be performed at a future point in time - This is handled by the 'Bottom Half'.

An interrupt handler is registered for an IRQ line using `request_irq()` which takes in information about the IRQ number, handler fn, flags pertaining to the nature of the interrupt and handler, and some extra stuff. The registration happens when the driver is loaded. Similarly, when the driver is unloaded the handler needs to be freed using `free_irq()`.

Interrupt handlers in linux need to be reentrant i.e the handler will not be invoked concurrently. When an interrupt is being service, the interrupt line is disabled (masked) which prevents further interrupts from coming on that line. Thus it is guaranteed that the ISR won't be invoked in parallel.

Interrupt lines may be shared among multiple handlers. For a line to be shared, each handler on that line must be registered as a shared handler. The handler returns a value denoting whether the interrupt was handled or not. When an interrupt is received on a shared line, the kernel invokes each of the handlers one by one. It uses the return value to ascertain whether the interrupt was handled.

Interrupt handlers run in the interrupt context. Since it is not backed by a process, ISR are not allowed to sleep (who will wake it up and how?), which restricts the activities that can be done from ISR. Earlier ISR was forced to use the stack of the process it interrupted. Now, there is an interrupt stack associated with the kernel which is of size equivalent to one page which the ISR can use.

`cat /proc/interrupts` shows the interrupt line, the number of interrupts received by each CPU, the interrupt controller, the interrupt type, and the device.

Bottom halves may be implemented using softirqs, tasklets, or work queues.


## Synchronization {#synchronization}

Locks are implemented using atomic test and set insturctions that are provided by the underlying architecture. Atomic operations using `atomic_t`. There's a lot to talk about here. In the book, which is based on linux kernel 2.6, `atomic_t` is implemented as a `volatile int` inside a struct. The struct was chosen so that there would be no way to cast it into another valid form. The choice of `volatile` does not seem to have survived the test of time, with more recent variants moving away from it altogether. This [document](https://www.kernel.org/doc/html/latest/core-api/atomic_ops.html) goes through the structure and behavior of the latest version of `atomic_t`, in addition, it also sheds light on [why volatile should not be used](https://www.kernel.org/doc/html/latest/process/volatile-considered-harmful.html#volatile-considered-harmful). Most of these arise because of the reliance on `volatile` to enforce a memory barrier while it actually does not. This [SO answer](https://stackoverflow.com/questions/246127/why-is-volatile-needed-in-c) illustrates cases where volatile can be justifiably used to prevent the compiler from optimising away checks and conditions that rely on MMIO.

`atomic_t` provides atomic operations to manipulate integers and bits. This is useful in cases where the critical region does not perform any operation more complicated than that. Locks are used when the critical regions are more complex, where multiple operations need to be performed while still ensuring atomicity.

Spin lock are a form of locking provided by the kernel, where the thread busy waits (spins) until the lock is acquired. This might seem inefficient in comparison to scenarios where the threads are put to sleep if the lock is not available. In cases where the locks are held for a short duration of time (or in code paths where you cannot sleep), spinlocks are efficient as it foregoes the overhead of scheduling involved with sleeping the thread and waking it up. An interesting fact is that the locks compile away in uniprocessor machines to markers that disable and enable kernel pre-emption. Interrupt handlers can use spin-locks, provided local interrupts to the current processor are disabled (this ensures that we do not get stuck with a double acquire deadlock). Lock data not code. Reader/writer variant of the spin lock is also provided by the kernel. A RW spin lock always favors readers. A sufficient number of readers could cause the starvation of the writer!

Semaphores in linux are sleeping locks. Can only be used in process context since it is not possible to sleep in an interrupt context (as they won't be rescheduled). Variants : binary semaphores (mutex) and counting semaphores. In addition, there's a rwsemaphore and a mutex(as a separate struct with a simpler interface).

There's another curious thing called the `completion variable` provided by the kernel. This is useful is scenarios where one process is waiting for a singal from the other indicating completion. A semaphore can be used here, but `completion variable` provides a simpler interface.

In addition to all of this there's the Big Kernel Lock (BKL) that was added to ease the SMP transition. It's a recursive, global spin lock that can be used in the process context. It's interesting to note how different projects got started with coarse grained locking, and later moved to fine grained locking as the project matured and the need for concurrency grew. I wonder when Python will tide over the Global Interpreter Lock(GIL).

Just when you thought you couldn't need another locking mechanism, the kernel throws `seqlocks` in your face. This is sort of like the RW locks seen earlier, with the difference being that writers are preferred over readers. Each locked data is associated with a sequence counter (which is the thing that's protected by the lock). During a write, the counter is incremented. A read operation checks the sequence counter prior to and after the read. A read succeeds(as in, a write did not happen in between), if the values match. Thus, writers are never blocked, and dirty reads would just cause the reader to retry the read until the counter conditions are satisfied.


## Ordering &amp; Barriers {#ordering-and-barriers}

The processor and compiler might reorder memory accesses (reads and write) in code for a variety of reasons which might break some implicit assumptions that the code relies on. Barriers can be used to enforce the ordering and to indicate to the compiler / processor to maintain the order of operations. `mb()/rmb()/wmb()` provide memory barrier / read memory barrier / write memory barrier which ensure that rw / reads / writes are not reordered across them i.e all corresponding ops before the barrier are guaranteed to complete before the ops after it.


## Memory Management {#memory-management}

Pages are treated as the smallest unit of memory management. Memory is divided in multiple zone, each zone with a particular characteristic (DMA, normal, high memory etc). Allocations will never cross zone boundaries. Pages returned to user-space are zeroed out to ensure security. `kmalloc` allocates memory that is physically contiguous while `vmalloc` allocates memory that is contiguous in the virtual address space.


## File Managements {#file-managements}

VFS abstraction layers allows userland programs to be agnostic of the underlying fs. Main components: superblock, inode, dentry, file.


## Block Devices {#block-devices}

Sectors, blocks, buffers, and buffer heads. The IO scheduler mergers and sorts requests on the block device to maximize "global" throughput. Anticipatory, deadline, completely fair queuing, and noop variant.


## <span class="org-todo todo TODO">TODO</span>  {#d41d8c}


### Interesting but not interesting enough for now {#interesting-but-not-interesting-enough-for-now}

-   Interrupt Handler Bottoms - softirq, tasklets
-   Slab allocator
