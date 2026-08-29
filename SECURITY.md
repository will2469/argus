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
* A detailed description of the vulnerability.
* Minimal reproducible Go code snippet or SQL migration demonstrating the issue.
* Any potential impact or attack vectors.
* (Optional) Proposed remediation or patch.

---

## Response Timeline & Process

* **Acknowledgment:** We aim to acknowledge receipt of your vulnerability report within **48 hours**.
* **Assessment:** We will validate the issue, assess severity, and provide a remediation timeline within **7 business days**.
* **Coordinated Disclosure:** A security patch will be prepared in a private fork and released alongside an official CVE advisory. We ask that you refrain from public disclosure until the release is published.
* **Credit:** We will publicly credit your contribution in the release notes and security advisory (unless you prefer to remain anonymous).
