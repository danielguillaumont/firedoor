# FD-004 10C3D2 Windows PowerShell 5.1 offline acceptance.
# Does NOT execute terraform, kubectl, Docker, KIND, or inspect private evidence.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3D2FreshPlan.ps1'
$expected = @(
    'docs/v10-fd004-fresh-plan-source-preparation.md',
    'internal/recoveryfreshplan/stage.go',
    'internal/recoveryfreshplan/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/cmd/fd004freshplanstage/main.go',
    'docs/v10-fd004-fresh-private-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D2FreshPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D2FreshPlan.ps1'
) | Sort-Object
function Assert-Exit([string]$Step) { if ($LASTEXITCODE -ne 0) { throw "$Step failed. Stop." } }
Push-Location -LiteralPath $repo
try {
    Write-Host '=== EXACT NINE-FILE SOURCE CHECKPOINT ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $dirty = @(& git status --porcelain --untracked-files=no); Assert-Exit 'tracked files'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'untracked files'
    if ($branch -cne 'feat/v10-fd004-fresh-plan-preparation' -or $head -cne '60a227d' -or
        $dirty.Count -ne 0 -or @(Compare-Object $expected $untracked).Count -ne 0) {
        throw 'Unexpected source inventory or commit. Stop.'
    }
    Write-Host 'PASS: Exactly nine source-only files at the merged base.'
    Write-Host '=== WINDOWS POWERSHELL SYNTAX AND PLAN-ONLY CONTRACT ==='
    foreach ($path in @($runner,$PSCommandPath)) {
        $tokens = $null; $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell syntax errors: $path" }
    }
    $src = Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach ($pattern in @('(?mi)^\s*&\s*terraform\s+(?:apply|destroy|import|state|force-unlock|workspace|taint|untaint)\b',
                           '(?mi)^\s*&\s*kubectl\s+(?:apply|create|delete|patch|edit|replace|rollout|scale|label|annotate|exec)\b',
                           '(?mi)^\s*&\s*kind\s+(?:create|delete)\b',
                           '(?mi)^\s*&\s*docker\s+(?:run|rm|start|stop|restart|kill)\b',
                           '(?mi)^\s*&\s*terraform\s+plan[^\r\n]*-out=\$[a-z]\w*\s*$')) {
        if ([regex]::IsMatch($src,$pattern)) { throw "Forbidden execution pattern: $pattern" }
    }
    foreach ($needle in @('"-out=$saved"','terraform init -backend=false -lockfile=readonly',
                          'terraform validate -no-color','terraform show -json $saved',
                          'fd004freshplanstage','fd004retainedplanadmit',
                          'fresh_plan_bound_to_authoritative_state = $false',
                          'apply_authorized = $false','[IO.FileMode]::CreateNew')) {
        if (-not $src.Contains($needle)) { throw "Missing plan safety boundary: $needle" }
    }
    Write-Host 'PASS: Parseable, guarded plan-only runner; no apply/destroy primitives.'

    Write-Host '=== REGRESSION: EXECUTE ACTUAL JSON HELPER WITH GO ARGUMENTS ==='
    # PowerShell's automatic $args can replace a function parameter named $Args.
    # Extract and invoke the helper from the exact runner under test (not a copy).
    $sourceTokens = $null
    $sourceErrors = @()
    $syntaxTree = [Management.Automation.Language.Parser]::ParseInput(
        $src, [ref]$sourceTokens, [ref]$sourceErrors)
    if ($sourceErrors.Count -ne 0) { throw 'Runner AST parsing failed. Stop.' }
    $helperNodes = @($syntaxTree.FindAll({
        param($node)
        ($node -is [Management.Automation.Language.FunctionDefinitionAst]) -and
        ($node.Name -ceq 'Get-Json')
    }, $true))
    if ($helperNodes.Count -ne 1) { throw 'Expected exactly one Get-Json helper. Stop.' }
    $helperText = $helperNodes[0].Extent.Text
    if ([regex]::IsMatch($helperText,'(?i)\$Args\b') -or
        -not [regex]::IsMatch($helperText,'\[string\[\]\]\$CommandArgs\b') -or
        -not [regex]::IsMatch($helperText,'&\s*\$Exe\s+@CommandArgs\b')) {
        throw 'Get-Json must splat an explicitly named nonautomatic argument vector. Stop.'
    }
    . ([scriptblock]::Create($helperText))
    $goEnvironment = Get-Json 'Go environment argument regression' 'go' @(
        'env','-json','GOOS','GOARCH')
    if ($goEnvironment.GOOS -cne 'windows' -or
        [string]::IsNullOrWhiteSpace([string]$goEnvironment.GOARCH)) {
        throw 'The actual Get-Json helper did not forward Go CLI arguments. Stop.'
    }
    Write-Host 'PASS: Native command subcommand and flags reached Go; JSON decoded.'
    Write-Host '=== NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'plan preview'
    Write-Host '=== FOREIGN CLUSTER REJECTION BEFORE PRIVATE ACCESS ==='
    $prior = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Plan `
            -ConfirmCluster firedoor-local -FreshPath C:\Nonexistent -ManifestSHA256 ('A' * 64) 2>&1 | Out-Null
        $rejected = $LASTEXITCODE
    } finally { $ErrorActionPreference = $prior }
    if ($rejected -eq 0) { throw 'Protected cluster was not rejected. Stop.' }
    Write-Host 'PASS: Protected cluster rejected before private access.'
    Write-Host '=== GO REGRESSION AND FULL SOURCE TESTS ==='
    $bad = @(& gofmt -l 'internal/recoveryfreshplan/stage.go' 'internal/recoveryfreshplan/stage_test.go')
    Assert-Exit 'gofmt'
    if ($bad.Count -ne 0) { throw 'Go source needs formatting. Stop.' }
    & go test ./internal/recoveryfreshplan -count=1; Assert-Exit 'fresh source Go tests'
    & go test ./scripts/v10/cmd/fd004freshplanstage; Assert-Exit 'fresh source Go verifier'
    & go test ./...; Assert-Exit 'full Go test suite'
    & go vet ./...; Assert-Exit 'go vet'
    $after = @(& git status --porcelain --untracked-files=no); Assert-Exit 'posttests Git source'
    if ($after.Count -ne 0) { throw 'Tests modified tracked source. Stop.' }
    Write-Host 'PASS: 10C3D2 offline acceptance.'
    Write-Host 'NO Terraform, Kubernetes, Docker, KIND, private evidence or state access.'
} finally { Pop-Location }
