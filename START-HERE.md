# Firedoor - first session on Windows PowerShell

This is a **starter ZIP**, not a claim that v0.0 is already verified or merged. The archive contains a single `firedoor/` directory; extract it into `$HOME\Projects`, not inside an existing repository. Later milestone ZIPs will be scoped changes, never blind overwrites.

## 1. Extract and inspect

```powershell
New-Item -ItemType Directory -Path "$HOME\Projects" -Force | Out-Null
Expand-Archive -LiteralPath "$HOME\Downloads\Firedoor-v00-Foundation-Starter.zip" -DestinationPath "$HOME\Projects"
Set-Location "$HOME\Projects\firedoor"
Get-ChildItem -Force
```

Use the actual download path if your browser saved the ZIP elsewhere. Do not extract again if this folder already contains work.

## 2. Inspect host tools (no installs or Kubernetes mutation)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\Check-Environment.ps1
go version
terraform version
git --version
```

For the full lab later, Docker Desktop, kind, kubectl and Node.js will also be required. v0.0 unit tests only need Git, Go and Terraform.

## 3. First dependency installation and validation

```powershell
go mod tidy
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\Test-Foundation.ps1
```

The first Go and Terraform dependency downloads need internet. If the validation fails, stop, copy the full output, and fix it before GitHub push. Running `terraform init -backend=false` is not `apply` and will not create a cluster.

## 4. Git and GitHub (after gates pass)

Create an empty **firedoor** GitHub repository, **without** pre-generated README/gitignore/license; select Git SSH. Then:

```powershell
git init -b main
git commit --allow-empty -m "chore: initialize Firedoor repository"
git switch -c feat/v00-foundation
git add .
git status --short
git diff --cached --check
git commit -m "feat: bootstrap Firedoor v0.0 foundation"
git remote add origin git@github.com:danielguillaumont/firedoor.git
git push -u origin main
git push -u origin feat/v00-foundation
```

Open a GitHub PR from `feat/v00-foundation` to `main` and merge **only after Actions passes**. Do not push any generated Terraform state, plans, tokens, kubeconfig, or `.terraform/` directory. Validate `git status --short` and `git ls-files` before push.

**First stop point:** after running Step 2, share the inventory and version output; verify all prerequisites before continuing.
