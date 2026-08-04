# AI Agent Development Rules & Guidelines

This document contains rules, style constraints, and behavioral guidelines for AI agents and coding assistants collaborating on the **RiderMate-Bot-Service (Gemini Live Client)** codebase. Adhering to these rules guarantees compilation safety, robust concurrency, frontend quality, and project coherence.

---

## 🏗️ 1. Codebase Architecture & Restructuring Constraints

*   **Production Code Location**: All core logic of the Gemini Live Client resides in [pkg/gemini/](pkg/gemini/). Avoid placing production client logic in the root directory.
*   **Compilation Stubs (Offline/Sandbox Environment)**: The top-level packages `llms/`, `tools/`, `utils/`, and `internal/` serve as stubs to emulate the parent application in an offline or sandboxed environment.
    *   *Rule*: If you modify the interface or method signatures in `pkg/gemini/`, you **must** update the corresponding stubs in these folders to preserve clean compilation.
    *   *Rule*: Never import upstream private packages directly if they require Git terminal authentication or private network paths in CI/CD. Emulate them locally.
*   **Sandbox Client**: Demonstrative examples or harness programs must reside in [cmd/sample-client/](cmd/sample-client/).

---

## ⚡ 2. Go Concurrency & Concurrency-Safe Rules

Because `GeminiLiveClient` drives a bidirectional, real-time voice stream with Voice Activity Detection (VAD), concurrency bugs can lead to audio lag, deadlocks, or goroutine leaks.

*   **State Tracker Safety**: Always use atomic operations (`sync/atomic`) or read-write locks (`sync.RWMutex`) when reading or modifying client state variables:
    *   `connected` (protected by `mu` RWMutex)
    *   `userSpeaking` (atomic `int32`)
    *   `responseInProgress` (atomic `int32`)
*   **Channel Safety**:
    *   Use explicit channel directional indicators in function signatures (e.g., `<-chan llms.AudioOutput` for read-only channels).
    *   Always verify that channel writes are non-blocking or guarded by context checks:
        ```go
        select {
        case c.audioOutput <- output:
        default:
            apiLogUtils.LogGeneralError("channel full, dropping message")
        }
        ```
*   **Goroutine Clean-up**:
    *   Every background goroutine spawned during `Connect()` (`receiveResponses`, `processEventQueue`, `handleWebSocketWrites`, `heartbeat`) **must** listen to the cancellation of `c.ctx.Done()` and exit cleanly.
    *   Ensure that `Close()` safely triggers context cancellation and calls `wg.Wait()` before terminating.

---

## 🎨 3. Frontend Development Rules & Guidelines (`frontend/`)

When working inside the `frontend/` directory, the following design and code quality constraints must be strictly followed:

*   **Technology Stack**: Use **PreactJS** with **Vite** for blazing fast, lightweight rendering. Write clean component code using functional components and hooks.
*   **Aesthetics & Style Constraints**:
    *   **Premium Visuals**: Build only highly immersive, cohesive dark themes. Use a unified HSL or CSS variable-based palette (neon blues/purples, charcoal grays, glassmorphism borders with `backdrop-filter: blur()`).
    *   **Micro-animations**: Integrate subtle transitions on hover, and pulsing glow indicators on connection states. Add an active voice wave visualizer when audio streaming is active.
    *   **No Placeholders**: Ensure all components are fully functional and use actual layout graphics or generated media assets.
*   **Web Audio API Constraints**:
    *   **Sampling Rate**: Always capture microphone input at **16kHz mono** to avoid unnecessary resamplers on the server side and minimize network bandwidth.
    *   **User Gesture Lock**: Browser privacy security locks the `AudioContext` initially. Always resume/start the `AudioContext` only after a direct user action (e.g. clicking "Connect" or "Start Stream").
    *   **Crackle-Free Playback**: Queue incoming binary chunk buffers sequentially into an active audio node sequence or queue to avoid popping/clicking audio issues.
*   **Quality & Lint Standards**:
    *   Keep all components modular and separated (e.g. `src/components/`, `src/utils/`).
    *   Use standard ESLint rules for code formatting and sanity checks. Ensure the build completes with zero lint or typescript errors.

---

## 🪵 4. Logging & Telemetry Protocols

*   **Standard Logger**: Never use standard `fmt.Printf`, `fmt.Println`, or the standard library `log` directly inside `pkg/gemini/`.
*   **Vetting Rule**: Always format informational and debugging telemetry through the package-local logger stubs:
    *   `apiLogUtils.LogGeneralInfo("format string %s", args...)`
    *   `apiLogUtils.LogGeneralError("format string %v", err)`

---

## 🧪 5. Automated Verification Standards

Before finalizing any code modification (backend or frontend), you **must** execute the following verification steps locally in order:

### Backend Checks
1.  **Format and Clean Imports**:
    ```bash
    go fmt ./...
    go mod tidy
    ```
2.  **Lint and Static Vetting**:
    ```bash
    go vet ./...
    ```
3.  **Execute Unit Tests**:
    ```bash
    go test -v ./...
    ```

### Frontend Checks
4.  **Install & Lint**: Ensure no linting errors exist:
    ```bash
    cd frontend
    npm run lint
    ```
5.  **Compile & Build**: Verify that the production bundles build successfully with zero errors:
    ```bash
    npm run build
    ```
6.  **Run Frontend Tests**: Execute tests if defined:
    ```bash
    npm run test
    ```
