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

This SDK handles JSON-RPC communication over stdio/WebSocket. Security considerations include:

- **Input Validation**: All JSON-RPC messages are validated against the protocol schema
- **Transport Security**: stdio transport uses local process communication; WebSocket transport (when implemented) should use TLS
- **Error Handling**: Errors are typed and wrapped to prevent information leakage
- **Concurrency**: All shared state is protected by mutexes to prevent data races

## Dependencies

This project has zero external dependencies outside of the Go standard library. All code is reviewed and tested for security issues.

## Contact

For security concerns, open an issue at https://github.com/dominicnunez/codex-sdk-go/issues.
