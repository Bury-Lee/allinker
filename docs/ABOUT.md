# What is ALLinker?

> A plain-language explanation for non-technical readers.

---

## One-Liner

**ALLinker is a small tool that helps multiple AI assistants work on the same project without stepping on each other's toes.**

---

## Why Was This Built?

Many AI coding assistants (like Cline, CodeX, Trae, etc.) can write code and edit files for humans. But there's a problem:

**When multiple AI assistants work in the same project at the same time, they interfere with each other.**

A real-world analogy:

> Imagine you ask two assistants to edit the same Word document at the same time:
>
> - Assistant A is editing page 3, Assistant B opens the same document to edit page 5
> - Assistant A saves, then Assistant B saves
> - Result: Assistant A's changes are overwritten. All that work is lost.
>
> To make it worse, the two assistants can't talk to each other. Neither knows what the other is doing.

Real projects are far more complex. ALLinker solves this problem.

---

## What Can It Do?

ALLinker provides four basic features — think of them as "rules" that AI assistants follow:

### 1. File Locking — "If I'm using it, nobody else can touch it"

Before editing a file, an AI assistant says "I'm locking this file." Other assistants see the lock and wait instead of editing the same file at the same time.

Like a sign-out sheet for a toolbox: you take the hammer, write your name down, and nobody else takes it until you bring it back.

### 2. Messaging — "They can talk to each other"

AI assistants can send messages to each other. For example:

- Assistant A tells Assistant B: "I'm done, your turn."
- Assistant A announces to everyone: "Nobody touch the XXX file, I'm still working on it."

It works just like @mentioning someone in a group chat.

### 3. File Watching — "Tell me when you're done"

An AI assistant can set up a "watchpoint": keep an eye on a specific folder, waiting for a specific file to appear.

For example:

- Assistant A gives a task to Assistant B: "Write the user registration feature and put the result in folder X."
- Assistant A sets up a watchpoint on folder X, waiting for the result.
- As soon as B puts the file there, A knows and continues working.

### 4. Account Management — "Who's who, and what are they allowed to do?"

Each AI assistant has its own identity account.

- **Admin**: Can create accounts, disable accounts, view everyone's operation history.
- **Agent**: Can do normal work.
- **Guest**: Can only view, not modify.

Every operation is logged. If something goes wrong, you can check who did what and when.

---

## How Is This Different From Other Tools?

You may have heard of MCP, A2A, AutoGen, CrewAI, or similar projects. ALLinker is not the same thing. Here are the three things that make it unique:

### 1. Wait-Driven Collaboration — "I'll wait for you, you wait for me"

Most multi-AI frameworks are **workflow-driven**: break a task into steps, assign each step to a different AI in sequence. It's like an assembly line — one AI finishes, the next one takes over.

But in reality, AI assistants often work **simultaneously and independently**: you don't know when the other will finish, so you can't plan a fixed sequence in advance.

ALLinker takes a **wait-driven** approach:

- One AI gives a task to another and **blocks until it's done** — no wasteful polling
- When the other finishes and drops the result file, ALLinker notifies whoever was waiting
- No need to pre-orchestrate a workflow. Whoever finishes first notifies the next person.

> Think of an office: you email a colleague saying "please finish this part," then you go about your day. When they're done, they notify you. Nobody has to keep asking "are you done yet?" That's wait-driven collaboration.

### 2. Peer-to-Peer — "Nobody is the boss"

Many multi-AI systems use a **master-worker architecture**: one "commander AI" directs a team of "worker AIs." The commander decides who does what and when.

ALLinker is different. **All AIs are equal peers:**

- No "leader AI" vs "worker AI" distinction
- Every AI can independently lock files, send messages, assign tasks, and wait for results
- No single point of failure — if one AI leaves, the others keep working

> It's like a team of colleagues where nobody is anyone's boss. Anyone can initiate collaboration. It's not "the leader delegates" — it's "everyone works it out together."

### 3. Cross-Tool — "Cline and CodeX, same table"

Existing "collaboration" features are usually locked inside a single tool. For example, Cursor's collaboration mode only works between Cursor instances.

ALLinker is **tool-agnostic**:

- Cline can collaborate with CodeX
- Trae can collaborate with Windsurf
- You can even run ALLinker commands manually to collaborate with AI assistants

As long as each tool supports running command-line programs, it can plug into ALLinker.

> Think of it like this: WeChat and iMessage can't talk to each other, but **SMS** works for everyone. ALLinker is the "SMS" for AI assistants — simple, universal, open to all.

---

## How Does It Run?

ALLinker has two modes:

### Local Mode

All AI assistants work on the same computer. ALLinker acts as a "middleman" coordinating communication and file access between them.

### Server Mode (LAN)

ALLinker runs as a service on one computer, and AI assistants on **other computers in the same local network** connect to it over the network. This allows multiple machines to collaborate.

---

## Do I Need to Know Programming?

**No.**

ALLinker is a small tool written in Go, but **users don't need to know how to program**. It's just an executable file — double-click or type one command and it runs. The AI assistants call it automatically.

---

## Where Is the Data Stored?

All data is stored in a hidden folder called `.alf/` in your project directory:

- Who registered
- Who locked which file
- Who sent what message to whom
- Who did what and when

Everything is logged and can be reviewed at any time. Data never leaves your computer — it's not uploaded to any cloud service.

---

## Is It Open Source?

Yes. ALLinker is licensed under **Apache License 2.0**. You are free to use, modify, distribute, and even use it commercially.

---

## Summary in Plain Language

> ALLinker is like a **site foreman** —
>
> When multiple AI assistants work on the same project at the same time, it makes sure:
>
> - Two people don't edit the same file at the same time 🔒
> - Assistants can talk to each other 💬
> - One assistant can wait for another to finish its task 👀
> - Everyone's identity and actions are recorded 📋
>
> Without it, AI assistants are like workers on the same site who can't see or talk to each other. Chaos is inevitable.
