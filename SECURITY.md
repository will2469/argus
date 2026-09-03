# Security Policy

The Argus team takes the security of our static analysis tools and database linters seriously. We appreciate responsible disclosure of security vulnerabilities.

---

## Supported Versions

Only the latest minor release branch receives security updates and bug fixes.

| Version | Supported          |
| :------ | :----------------- |
| `v1.x`  | :white_check_mark: |
| `< 1.0` | :x:                |

---

## Tool Description Policy

Argus MCP tools follow a **descriptive, not imperative** policy for tool descriptions to prevent "tool steering" or "tool description poisoning" - a pattern where tool metadata is used to imperatively command AI agents rather than describe functionality.

### Design Rationale

- **Avoid Imperative Language:** Tool descriptions describe what a tool does, not when/how it MUST be invoked
- **No Agent Steering:** We do not use tool descriptions to force agents to call specific tools
- **Transparent Functionality:** Descriptions focus on capabilities, use cases, and limitations
- **User Agency:** Agents and users make informed decisions about tool invocation based on needs

### Implementation Guidelines

All Argus MCP tool descriptions:

- Use descriptive language ("Database safety auditor", "Analyzes Go code")
- Avoid imperative commands ("You MUST invoke", "MANDATORY", "REQUIRED")
- Provide use case guidance when helpful ("Recommended for use after writing database code")
- Document capabilities, limitations, and parameters accurately

This policy prevents malicious MCP servers from abusing tool descriptions to force unwanted tool calls and respects user/agent agency in tool selection.

---

## Content Spoofing Prevention

Argus implements content spoofing protections in telemetry/formatter.go to prevent user-provided code snippets from breaking markdown structure in GitHub issues.

### Code Fence Escaping

The `escapeCodeFence()` function prevents markdown content spoofing by breaking code fence sequences (```) within user-provided snippets:

- Replaces triple backticks with backtick + zero-width space + backtick + backtick
- Prevents malicious snippets from terminating code fences and spoofing subsequent sections
- Preserves visual rendering while breaking fence sequence parsing

### Security Impact

This prevents:

- Content spoofing where malicious snippets can visually corrupt issue metadata
- Structured injection attacks where snippets can break out of code blocks
- Visual confusion in GitHub issues where attacker-controlled content appears as legitimate metadata

GitHub sanitizes HTML in issue bodies, so this is a content integrity fix rather than an XSS/RCE vector.

---

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability in Argus (such as an arbitrary code execution vector, malicious AST exploitation, bypass of security rules, or denial-of-service), please report it responsibly using one of the following methods:

### Option 1: GitHub Private Vulnerability Reporting (Preferred)

Submit a report through GitHub's private vulnerability advisory dashboard:
👉 **[Report a Security Vulnerability](https://github.com/will2469/argus/security/advisories/new)**

This allows you to collaborate directly with the maintainers in a private, encrypted workspace.

### Option 2: Direct Email

Send an email with the subject line `[SECURITY] Vulnerability in Argus` to:
📧 **[will.i.is.ega@gmail.com](mailto:will.i.is.ega@gmail.com)**

Please include:

- A detailed description of the vulnerability.
- Minimal reproducible Go code snippet or SQL migration demonstrating the issue.
- Any potential impact or attack vectors.
- (Optional) Proposed remediation or patch.

---

## Response Timeline & Process

- **Acknowledgment:** We aim to acknowledge receipt of your vulnerability report within **48 hours**.
- **Assessment:** We will validate the issue, assess severity, and provide a remediation timeline within **7 business days**.
- **Coordinated Disclosure:** A security patch will be prepared in a private fork and released alongside an official CVE advisory. We ask that you refrain from public disclosure until the release is published.
- **Credit:** We will publicly credit your contribution in the release notes and security advisory (unless you prefer to remain anonymous).
