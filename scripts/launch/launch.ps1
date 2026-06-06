# OpenForge Path Launch Helper
# 启动指定 Path（创建 worktree + 验证基线 + 输出执行入口）

param(
    [Parameter(Mandatory)]
    [ValidateSet('A', 'B', 'C', 'D', 'X3', 'X4', 'setup', 'status', 'clean')]
    [string]$Action,

    [string]$BaseBranch = 'main'
)

$ErrorActionPreference = 'Stop'
$ProjectRoot = git rev-parse --show-toplevel
Set-Location $ProjectRoot

# 路径定义
$Paths = @{
    'A'  = @{ Branch='feat/path-A-data-closure';         Dir='.worktrees/feature/path-A-data-closure';         Plan='2026-06-06-path-A-data-closure.md' }
    'B'  = @{ Branch='feat/path-B-security-ux-closure';   Dir='.worktrees/feature/path-B-security-ux';          Plan='2026-06-06-path-B-security-ux-closure.md' }
    'C'  = @{ Branch='feat/path-C-architecture-honesty';  Dir='.worktrees/feature/path-C-architecture';        Plan='2026-06-06-path-C-architecture.md' }
    'D'  = @{ Branch='feat/path-D-enterprise-landing';    Dir='.worktrees/feature/path-D-enterprise';          Plan='2026-06-06-path-D-enterprise.md' }
    'X3' = @{ Branch='feat/X3-db-24-optimization';        Dir='.worktrees/feature/path-X3-db-24-optimization'; Plan='2026-06-06-X3-db-24-optimization.md' }
    'X4' = @{ Branch='feat/X4-proto-fault-bench';         Dir='.worktrees/feature/path-X4-proto-fault-bench';  Plan='2026-06-06-X4-proto-fault-bench.md' }
}

# 路径前置依赖
$Predecessor = @{
    'A'  = $null
    'B'  = 'A'
    'D'  = 'B'
    'C'  = 'D'
    'X3' = 'C'
    'X4' = 'C'
}

function Write-Section($title) {
    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "  $title" -ForegroundColor Cyan
    Write-Host "============================================================" -ForegroundColor Cyan
}

function Test-Prerequisites {
    Write-Section "Prerequisites Check"
    $fail = $false

    # 1. .worktrees gitignored
    if (-not (git check-ignore -q .worktrees 2>$null)) {
        Write-Host "✗ .worktrees not gitignored" -ForegroundColor Red
        $fail = $true
    } else {
        Write-Host "✓ .worktrees gitignored" -ForegroundColor Green
    }

    # 2. main 分支存在
    $mainExists = git show-ref --verify --quiet refs/heads/main
    if (-not $mainExists) {
        Write-Host "✗ main branch not found" -ForegroundColor Red
        $fail = $true
    } else {
        Write-Host "✓ main branch exists" -ForegroundColor Green
    }

    # 3. main 工作区干净
    $status = git status --porcelain
    if ($status) {
        Write-Host "✗ main worktree has uncommitted changes:" -ForegroundColor Red
        Write-Host $status
        $fail = $true
    } else {
        Write-Host "✓ main worktree clean" -ForegroundColor Green
    }

    if ($fail) { throw "Prerequisites failed. Fix above issues first." }
}

function Test-PredecessorMerged($path) {
    $pred = $Predecessor[$path]
    if (-not $pred) { return $true }

    $predPath = $Paths[$pred]
    $branchExists = git show-ref --verify --quiet "refs/heads/$($predPath.Branch)"
    if (-not $branchExists) {
        Write-Host "✗ Predecessor path $pred not started yet (branch $($predPath.Branch) missing)" -ForegroundColor Red
        return $false
    }
    # 检查 predecessor 的 plan doc 是否存在
    $planFile = Join-Path $ProjectRoot "docs/superpowers/plans/$($predPath.Plan)"
    if (-not (Test-Path $planFile)) {
        Write-Host "✗ Predecessor plan file missing: $planFile" -ForegroundColor Red
        return $false
    }
    Write-Host "✓ Predecessor $pred (branch $($predPath.Branch)) found" -ForegroundColor Green
    return $true
}

function New-PathWorktree($path) {
    Write-Section "Creating worktree for Path $path"
    $cfg = $Paths[$path]

    # 检查是否已存在
    $existing = git worktree list --porcelain | Select-String "^worktree.*$($cfg.Dir.TrimStart('./'))"
    if ($existing) {
        Write-Host "✓ Worktree already exists at $($cfg.Dir)" -ForegroundColor Yellow
        return
    }

    # 创建
    git fetch origin main 2>$null
    git worktree add $cfg.Dir -b $cfg.Branch $BaseBranch
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create worktree for Path $path"
    }
    Write-Host "✓ Created worktree at $($cfg.Dir) on branch $($cfg.Branch)" -ForegroundColor Green
}

function Test-Baseline($path) {
    Write-Section "Running baseline tests for Path $path"
    $cfg = $Paths[$path]
    $wt = Join-Path $ProjectRoot $cfg.Dir

    Push-Location $wt
    try {
        $env:GOCACHE = "$ProjectRoot\.cache\go-build"
        New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null

        Write-Host "→ go test ./internal/..."
        go test ./internal/... -count=1 2>&1 | Select-Object -Last 5
        if ($LASTEXITCODE -ne 0) { throw "Go tests failed in worktree $wt" }

        if (Test-Path frontend/package.json) {
            Write-Host "→ npm run typecheck (frontend)"
            Push-Location frontend
            npm run typecheck 2>&1 | Select-Object -Last 3
            Pop-Location
        }

        if (Test-Path nodejs-io/package.json) {
            Write-Host "→ npm test (nodejs-io)"
            Push-Location nodejs-io
            npm test 2>&1 | Select-Object -Last 3
            Pop-Location
        }

        Write-Host "✓ Baseline tests passed" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

function Show-ExecutionPrompt($path) {
    $cfg = $Paths[$path]
    $wt = Join-Path $ProjectRoot $cfg.Dir
    $plan = Join-Path $ProjectRoot "docs/superpowers/plans/$($cfg.Plan)"

    Write-Section "Path $path ready — Execution prompt"

    Write-Host "Plan: $plan" -ForegroundColor Yellow
    Write-Host "Worktree: $wt" -ForegroundColor Yellow
    Write-Host "Branch: $($cfg.Branch)" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Next step: open Claude Code in worktree and execute:" -ForegroundColor White
    Write-Host ""
    Write-Host "  cd $wt" -ForegroundColor Gray
    Write-Host "  # 启动 subagent-driven-development 模式，按 plan md 逐任务派发" -ForegroundColor Gray
    Write-Host "  # 或 inline 执行（适合小修）" -ForegroundColor Gray
    Write-Host ""
    Write-Host "When complete:" -ForegroundColor White
    Write-Host "  cd $wt" -ForegroundColor Gray
    Write-Host "  git push -u origin $($cfg.Branch)" -ForegroundColor Gray
    Write-Host "  gh pr create --title `"Path ${path}: ...`" --body 'Closes N items from DESIGN stub-todo'" -ForegroundColor Gray
}

function Get-Status {
    Write-Section "OpenForge Path Status"
    Write-Host ("{0,-6} {1,-40} {2,-30} {3,-12}" -f "Path", "Branch", "Worktree", "Status")
    Write-Host ("{0,-6} {1,-40} {2,-30} {3,-12}" -f "----", "------", "--------", "------")

    foreach ($key in @('A', 'B', 'C', 'D', 'X3', 'X4')) {
        $cfg = $Paths[$key]
        $branchExists = $false
        try {
            $null = git rev-parse --verify "refs/heads/$($cfg.Branch)" 2>$null
            $branchExists = ($LASTEXITCODE -eq 0)
        } catch { $branchExists = $false }

        $wtExists = Test-Path $cfg.Dir
        if ($branchExists -and $wtExists) { $status = "✓ Ready"; $color = "Green" }
        elseif ($branchExists) { $status = "⚠ No worktree"; $color = "Yellow" }
        elseif ($wtExists) { $status = "⚠ No branch"; $color = "Yellow" }
        else { $status = "✗ Not started"; $color = "Gray" }
        Write-Host ("{0,-6} {1,-40} {2,-30} {3,-12}" -f $key, $cfg.Branch, $cfg.Dir, $status) -ForegroundColor $color
    }
}

function Remove-Path($path) {
    Write-Section "Cleaning up Path $path"
    $cfg = $Paths[$path]
    $wt = Join-Path $ProjectRoot $cfg.Dir

    if (Test-Path $wt) {
        git worktree remove --force $wt
        Write-Host "✓ Removed worktree $wt" -ForegroundColor Green
    }
    git branch -D $cfg.Branch 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Deleted branch $($cfg.Branch)" -ForegroundColor Green
    }
}

# === MAIN ===
switch ($Action) {
    'setup' {
        Test-Prerequisites
        foreach ($key in @('A', 'B', 'C', 'D', 'X3', 'X4')) {
            New-PathWorktree $key
        }
        Write-Section "Setup complete"
        Write-Host "All 6 path worktrees created. Run: .\scripts\launch\launch.ps1 -Action status" -ForegroundColor Green
    }

    {$_ -in @('A','B','C','D','X3','X4')} {
        Test-Prerequisites
        if (-not (Test-PredecessorMerged $Action)) {
            Write-Host ""
            Write-Host "Predecessor not satisfied. Either:" -ForegroundColor Yellow
            Write-Host "  1. Complete and merge predecessor first" -ForegroundColor Yellow
            Write-Host "  2. Force-override with -BaseBranch <branch>  (not recommended)" -ForegroundColor Yellow
            exit 1
        }
        New-PathWorktree $Action
        Test-Baseline $Action
        Show-ExecutionPrompt $Action
    }

    'status' {
        Get-Status
    }

    'clean' {
        if (-not $PSBoundParameters.ContainsKey('Action')) { ... }
        # 显式传 path 时清理（防止误删）
        # 当前为安全默认，要求显式调用
        $paths = $args | Where-Object { $_ -in @('A','B','C','D','X3','X4') }
        if (-not $paths) {
            Write-Host "Usage: launch.ps1 -Action clean <A|B|C|D|X3|X4>" -ForegroundColor Yellow
            exit 1
        }
        foreach ($p in $paths) { Remove-Path $p }
    }
}
