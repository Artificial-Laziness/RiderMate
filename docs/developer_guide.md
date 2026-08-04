# Developer Setup and Contribution Guide — RiderMate

Welcome! This guide will help you set up, develop, test, and contribute to the `RiderMate-Bot-Service` Gemini Live Client repository.

---

## 1. Prerequisites & Installation

To develop in this repository, you need:
*   **Go**: Version `1.26.5` or higher.
*   **Git**: For version control.

Verify your environment setup:
```bash
go version
git status
```

---

## 2. Directory Structure

This project follows Go repository best practices, separating library code, stubs, and runnable tools:

*   `pkg/gemini/`: Contains the production `GeminiLiveClient` codebase.
*   `cmd/sample-client/`: A runnable mock client showing how to instantiate the client and wire up callback structures for delivery rider voice sessions.
*   `llms/`, `tools/`, `utils/`, `internal/`: Local compilation stubs that represent the parent project's internal packages, allowing for isolated compilation and testing without private repo dependencies.
*   `docs/`: Design and architectural documents.

---

## 3. Getting Started

### Clone and Fetch Dependencies
Since all dependencies are configured via Go Modules, you can initialize, tidy, and download packages in a single step:
```bash
go mod tidy
```

### Run the Sample Client
We have provided a demo client under `cmd/sample-client` to showcase client initialization and mock callbacks. Run it locally using:
```bash
go run cmd/sample-client/main.go
```

**Expected Output:**
```
Starting RiderMate Gemini Live Client Sample...
[Mock Conversation] AddAssistantMessage: Provide hands-free delivery rider support. (text)
RiderMate Gemini Live Client created successfully: model=RiderMate
Rider voice session and VAD setup initialized.
```

---

## 4. Running Quality Checks

To ensure all code conforms to strict safety standards, run the following automated checks locally before pushing your changes:

### A. Run Linting & Static Vetting
```bash
go vet ./...
```

### B. Run Unit Tests
We use Go's standard testing framework. To execute all unit tests and view verbose details:
```bash
go test -v ./...
```

---

## 5. Mocking Missing Dependencies

If you need to introduce new dependencies from the parent `RiderMate-Bot-Service` project:
1.  **Do not import external private modules directly** if it breaks open development or requires private keys in CI.
2.  Instead, place standard stub equivalents in the local mock package folders (`llms`, `tools`, `utils`).
3.  Ensure they expose the matching signatures used in `pkg/gemini/gemini.go`.
4.  Run `go build ./...` to guarantee that your modifications do not break local compilation.

---

## 6. GitHub Contribution & Collaboration Flow

To collaborate with the team on GitHub, please follow this flow:

1.  **Create a Feature Branch**:
    ```bash
    git checkout -b feature/your-feature-name
    ```
2.  **Commit Your Changes**: Follow clear commit messages:
    ```bash
    git commit -m "feat(gemini): add support for rider emergency hands-free voice shortcuts"
    ```
3.  **Validate Locally**: Run `go vet ./...` and `go test ./...` to ensure everything is building and passing.
4.  **Push and Open a PR**:
    ```bash
    git push origin feature/your-feature-name
    ```
    Open a Pull Request on GitHub. The pre-configured **CI/CD workflow** will automatically validate your code.
