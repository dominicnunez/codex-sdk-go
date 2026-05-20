# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it by opening a GitHub issue at:

https://github.com/dominicnunez/codex-sdk-go/issues

When reporting a vulnerability, please include:

- A clear description of the vulnerability
- Steps to reproduce the issue
- Potential impact of the vulnerability
- Any suggested fixes or mitigations (if applicable)

## Security Updates

Security fixes will be released as patch versions following semantic versioning. Users are encouraged to keep their dependencies up to date.

## Scope

This SDK handles typed JSON-RPC request, response, and notification data, optional stdio app-server process management, and Codex login credentials. Security considerations include:

- **Input Validation**: Core request payloads and typed responses enforce required-field validation during decoding, but callers must still treat inbound notifications and unknown future fields as untrusted input
- **Transport Security**: callers are responsible for choosing and securing the transport implementation used with the SDK; stdio helpers only frame local process I/O
- **Credential Handling**: login helpers store credentials with restrictive file permissions and redact secrets from string formatting, but callers must not log raw credential fields or pass secrets via command-line arguments
- **Process Execution**: app-server process helpers require an absolute binary path to avoid PATH hijacking and use environment variables, not command-line arguments, for sensitive values
- **Error Handling**: Errors are typed and wrapped to prevent information leakage
- **Concurrency**: All shared state is protected by mutexes to prevent data races

## Dependencies

This project has zero external dependencies outside of the Go standard library. All code is reviewed and tested for security issues.

## Contact

For security concerns, open an issue at https://github.com/dominicnunez/codex-sdk-go/issues.
