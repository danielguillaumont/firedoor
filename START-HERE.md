# Firedoor — start here (Windows / PowerShell 5.1)

This is the **current v0.6 source repository**, not the original v0.0 starter ZIP. If you already have a checkout at `C:\Users\<you>\Projects\firedoor`, **do not extract another foundation ZIP, re-run `git init`, or overwrite your local Terraform state**. The original foundation instructions are preserved by earlier source history.

Firedoor contains two separate operator workflows: a **safe, read-only local browser demo** and a **dedicated owned kind/Calico security lab**. Start with the browser; you need no cluster for that.

## A. Read-only browser demo (recommended first)

Prerequisites: Windows PowerShell 5.1, Git, Go **1.27**, Node.js **22+**, npm. The default console does not need Docker, kind, Kubernetes credentials or Terraform.

```powershell
cd "$HOME\Projects\firedoor"
go version
node --version
npm --version
go mod download
cd .\apps\console
npm ci
```

Open two named Windows Terminal tabs:

**`api`**

```powershell
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor console-api serve
```

**`react`**

```powershell
cd "$HOME\Projects\firedoor\apps\console"
npm run dev
```

Visit `http://127.0.0.1:5173/`. Overview, Scenarios, Controls and API Capabilities can load default read-only contracts; Proof Lens, Environment and History remain **unavailable without separate approval**. Keep the server tabs open; use `Ctrl+C` to stop each one. A third tab, **`other`**, can run Git and quality checks.

```powershell
# Third terminal (`other`), from the repository root
cd "$HOME\Projects\firedoor"
go test ./...
go vet ./...
go mod verify
cd .\apps\console
npm run typecheck
npm test
npm run build
```

For an exact screenshot-ready demo and optional historical approval, follow the [v0.6 demo guide](docs/v06-console-demo-guide.md). The optional detached historical report is **not distributed with the repository** and must not be uploaded or committed.

## B. Dedicated controlled Kubernetes lab (separate, optional)

The earlier v0.1–v0.5 CLI workflows require additional installed prerequisites, a positively verified owned kind context, Docker Desktop, kubectl, Terraform and explicit operator safeguards. Never run bounded probe, A/B/A policy experiments, Terraform apply/destroy or evidence collection just to make the console load. Start with [SECURITY.md](SECURITY.md), [architecture](docs/architecture.md), and the [v0.3 safety boundaries](docs/v03-safety-boundaries.md) before any such activity.

## C. Git checkpoint discipline

The current v0.6 development work belongs on `feat/v06-console`; use `git status --short --branch` before applying each milestone ZIP. Validate first, stage only reviewed files, run `git diff --cached --check`, and push only after successful tests. `docs/evidence/local/` and Terraform state remain Git-ignored. Do not merge the v0.6 branch until the final PR and GitHub CI review is completed.

Read the [main README](README.md) for the full product and source map.
