# Gemini Live Client Integration — RiderMate

A standard, highly optimized Go implementation of the **Gemini Live Client** designed for bidirectional real-time audio and text streaming. This module acts as the core conversational logic driver for the `RiderMate-Bot-Service`, enabling hands-free voice assistance for delivery riders (e.g., active order updates, store delays, delivery route assistance, payout queries, and rider support escalation) to run on low-latency, full-duplex pathways powered by **Google Cloud Vertex AI** (v1beta1).

---

## 🗺️ Project Navigation Index

Welcome! Use this index to navigate the project documentation and resources:

*   **📚 Documentation**
    *   [System Architecture Guide](docs/architecture.md) — Dive deep into the concurrency pipeline, queue structures, and Voice Activity Detection (VAD) interrupt mechanics for delivery rider voice sessions.
    *   [Developer Setup & Contribution Guide](docs/developer_guide.md) — Quickstart tutorial, testing workflow, and GitHub contribution guidelines.
*   **🛠️ Core Codebase**
    *   [pkg/gemini/gemini.go](pkg/gemini/gemini.go) — The production `GeminiLiveClient` implementation.
    *   [pkg/gemini/gemini_test.go](pkg/gemini/gemini_test.go) — Automated unit tests validating initialization.
*   **🚀 Test Sandbox & Runnable Demo**
    *   [cmd/sample-client/main.go](cmd/sample-client/main.go) — Standard runnable client harness demonstrating how to bootstrap and consume callbacks for rider assistance.
*   **📂 Compilation Stubs (Sandbox Environment)**
    *   [llms/](llms/stubs.go), [tools/](tools/stubs.go), [utils/](utils/apiLogUtils/stubs.go), [internal/](internal/httprequest/httprequest.go) — Standalone stub packages mimicking upstream imports, ensuring **100% successful local compilation and CI/CD validation**.

---

## 🚀 Quick Onboarding

Get the project built, vetted, and running in less than a minute:

### 1. Download Dependencies
```bash
go mod tidy
```

### 2. Run Local Verification
```bash
go vet ./...
go test -v ./...
```

### 3. Start the Sample Sandbox Client
```bash
go run cmd/sample-client/main.go
```

---

## 🤖 CI/CD Integration

This repository is configured with a basic **GitHub Actions CI Pipeline** located at [ci.yml](.github/workflows/ci.yml). 

Upon every `push` and `pull_request` to the main branches, the pipeline automatically:
1.  Sets up a clean Go container.
2.  Fetches external modules.
3.  Performs strict static analysis vetting (`go vet`).
4.  Executes unit tests (`go test`).
5.  Validates clean compilation (`go build`).

This ensures high code quality, blocks broken commits, and enables frictionless developer collaboration on GitHub!
