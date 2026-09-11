# Security Policy

## Reporting a vulnerability

**Do not open a public issue.** Use one of:

- [GitHub Security Advisory](https://github.com/arandu-io/ayra/security/advisories/new)

That is the only channel, and it is deliberate. An `security@` address was
published here before the domain had a mail exchanger, so a report sent to it
reached nobody while the sender believed it had arrived — and the embargo clock
started running on its own. A channel that swallows the message is worse than no
channel at all.

The advisory form is private, it notifies the maintainers, and it is where the
fix and the disclosure happen.

A public issue about a vulnerability will be closed and moved to a private
advisory — with thanks, not with a scolding.

## What to expect, in time

| Step | Deadline |
|---|---|
| Acknowledgement | 72 hours |
| Triage and severity (CVSS) | 7 days |
| Fix, or a plan with a date | 30 days |
| Coordinated disclosure | 90 days, or sooner once a fix exists |

If the flaw is being exploited, the embargo ends: fix and notice go out
immediately.

## Supported versions

The current major and the previous one, for 12 months after the new major is
released. Security fixes land in both. While the project is on `v0.x`, only the
latest minor is supported.

## What counts as a vulnerability in this project

This library draws and it talks to a server. It decides nothing about who may
see what, so the classes of flaw are the ones that follow from those two jobs.

**Ours:**

- **The session store.** What it keeps is what proves who somebody is. A
  session written where another account can read it, left behind after signing
  out, carried past the life the server gave it, or handed to a server other
  than the one that issued it, is a vulnerability here.
- **The client.** A request that leaves without the session, reaches a host it
  was not pointed at, or accepts an answer it should have refused.
- **What packaging produces.** An artifact signed with the wrong key, an
  identifier that lets one application read another's data, or a permission
  declared in a manifest that the application does not use.
- **The engine beneath this library.** It is a fork and it is maintained here,
  so a flaw in it is ours to fix rather than to forward -- including one
  inherited from upstream and one arriving through a dependency it carries.
- **A control that shows what it was told to hide.** A password field that
  draws its characters, a value that survives a screen it was cleared on.

**Not ours:** what the server decides. This library carries no policy, evaluates
no permission and holds no data path, so an application that authorized
something it should not have has a flaw on the server side. Nor is a missing
accessible tree a vulnerability: eight of the nine platform backends do not walk
one, that is written on the first screen of the README, and it is a declared
boundary rather than a hole.

## Disclosure

Through GitHub Security Advisory, which issues a CVE. The advisory is published
together with the fixed version — never before it.

Every advisory states: affected versions, fixed versions, impact, temporary
mitigation, and credit to the reporter.

## Supply chain

- This module carries eight direct dependencies and every one of them belongs to
  the engine it vendors. A test names them and fails on a ninth, and fails as
  well on a name in the list that nothing requires any more
- `govulncheck` runs in CI over both modules and blocks the merge
- Nothing is pinned to a pseudo-version, which a test enforces: a dependency
  fixed to a commit rather than a tag is one nobody can audit by version
- Dependency updates are reviewed by a person; never merged automatically
- No `replace` pointing outside the module in a release

## After an incident

A fix has two parts: the patch, and something that stops the pattern coming
back. Here that is a test rather than a rule in a generator -- this library has
no generator -- and the test is written before the patch, so that it fails first.
