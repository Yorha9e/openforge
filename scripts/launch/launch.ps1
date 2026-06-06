# OpenForge Path Launch Helper (simplified, ASCII-only)
# Usage:
#   launch.ps1 status                 - show all 6 paths status
#   launch.ps1 A                      - bootstrap Path A worktree + baseline tests
#   launch.ps1 setup                  - create all 6 worktrees
#   launch.ps1 clean <A|B|...>        - remove a path worktree+branch

param(
    [Parameter(Mandatory, Position=0)]
    [string]$Action
)

$ErrorActionPreference = 'Stop'
$ProjectRoot = git rev-parse --show-toplevel
Set-Location $ProjectRoot

# Path definitions: key -> { Branch, Dir, Plan }
$Paths = [ordered]@{
    'A'  = @{ Branch='feat/path-A-data-closure';         Dir='.worktrees/feature/path-A-data-closure';         Plan='2026-06-06-path-A-data-closure.md' }
    'B'  = @{ Branch='feat/path-B-security-ux-closure';   Dir='.worktrees/feature/path-B-security-ux';          Plan='2026-06-06-path-B-security-ux-closure.md' }
    'C'  = @{ Branch='feat/path-C-architecture-honesty';  Dir='.worktrees/feature/path-C-architecture';        Plan='2026-06-06-path-C-architecture.md' }
    'D'  = @{ Branch='feat/path-D-enterprise-landing';    Dir='.worktrees/feature/path-D-enterprise';          Plan='2026-06-06-path-D-enterprise.md' }
    'X3' = @{ Branch='feat/X3-db-24-optimization';        Dir='.worktrees/feature/path-X3-db-24-optimization'; Plan='2026-06-06-X3-db-24-optimization.md' }
    'X4' = @{ Branch='feat/X4-proto-fault-bench';         Dir='.worktrees/feature/path-X4-proto-fault-bench';  Plan='2026-06-06-X4-proto-fault-bench.md' }
}

# Path predecessors
$Pred = @{ 'A'=$null; 'B'='A'; 'D'='B'; 'C'='D'; 'X3'='C'; 'X4'='C' }

function Section($title) {
    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "  $title" -ForegroundColor Cyan
    Write-Host "============================================================" -ForegroundColor Cyan
}

function Test-Prereqs {
    Section "Prerequisites Check"
    $ok = $true

    git check-ignore .worktrees *> $null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[X] .worktrees not gitignored" -ForegroundColor Red
        $ok = $false
    } else {
        Write-Host "[OK] .worktrees gitignored" -ForegroundColor Green
    }

    git rev-parse --verify refs/heads/main *> $null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[X] main branch not found" -ForegroundColor Red
        $ok = $false
    } else {
        Write-Host "[OK] main branch exists" -ForegroundColor Green
    }

    $trackedChanges = git status --porcelain | Where-Object { -not $_.StartsWith('??') }
    if ($trackedChanges.Count -gt 0) {
        Write-Host "[X] main has modified tracked files:" -ForegroundColor Red
        $trackedChanges | ForEach-Object { Write-Host "    $_" }
        $ok = $false
    } else {
        Write-Host "[OK] main worktree tracked files clean" -ForegroundColor Green
    }

    if (-not $ok) { throw "Prerequisites failed" }
}

function New-Wt($key) {
    Section "Creating worktree for Path $key"
    $cfg = $Paths[$key]

    if (Test-Path $cfg.Dir) {
        Write-Host "[skip] worktree already exists at $($cfg.Dir)" -ForegroundColor Yellow
        return
    }

    $null = git worktree add $cfg.Dir -b $cfg.Branch main 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "git worktree add failed for Path $key"
    }
    Write-Host "[OK] created worktree $($cfg.Dir) on branch $($cfg.Branch)" -ForegroundColor Green
}

function Test-Baseline($key) {
    Section "Running baseline tests for Path $key"
    $cfg = $Paths[$key]
    $wt = Join-Path $ProjectRoot $cfg.Dir

    Push-Location $wt
    try {
        $env:GOCACHE = Join-Path $ProjectRoot '.cache\go-build'
        if (-not (Test-Path $env:GOCACHE)) {
            New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
        }

        # 1. Regenerate protobuf (gen/ is gitignored)
        if (Test-Path 'buf.gen.yaml') {
            Write-Host "  -> buf generate proto"
            & buf generate proto *> $null
            if ($LASTEXITCODE -ne 0) { throw "buf generate failed" }
        }

        # 2. Go tests
        Write-Host "  -> go test ./internal/... -count=1"
        & go test ./internal/... -count=1 *> $null
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[X] Go tests FAILED in $wt" -ForegroundColor Red
            & go test ./internal/... -count=1 2>&1 | Select-Object -Last 20
            throw "Go tests failed"
        }
        Write-Host "[OK] Go tests passed" -ForegroundColor Green

        # 3. Frontend typecheck (best-effort)
        if (Test-Path 'frontend/package.json') {
            Write-Host "  -> frontend npm run typecheck"
            Push-Location frontend
            & npm run typecheck *> $null
            $typecheckOk = ($LASTEXITCODE -eq 0)
            Pop-Location
            if ($typecheckOk) {
                Write-Host "[OK] frontend typecheck passed" -ForegroundColor Green
            } else {
                Write-Host "[WARN] frontend typecheck had errors" -ForegroundColor Yellow
            }
        }

        # 4. nodejs-io tests (best-effort)
        if (Test-Path 'nodejs-io/package.json') {
            Write-Host "  -> nodejs-io npm test"
            Push-Location nodejs-io
            & npm test *> $null
            $nodeOk = ($LASTEXITCODE -eq 0)
            Pop-Location
            if ($nodeOk) {
                Write-Host "[OK] nodejs-io tests passed" -ForegroundColor Green
            } else {
                Write-Host "[WARN] nodejs-io tests had errors" -ForegroundColor Yellow
            }
        }

        Write-Host "[OK] Baseline ready for Path $key" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

function Show-Prompt($key) {
    $cfg = $Paths[$key]
    $wt = Join-Path $ProjectRoot $cfg.Dir
    $plan = Join-Path $ProjectRoot "docs/superpowers/plans/$($cfg.Plan)"

    Section "Path $key ready"

    Write-Host "  Plan:     $plan" -ForegroundColor Yellow
    Write-Host "  Worktree: $wt" -ForegroundColor Yellow
    Write-Host "  Branch:   $($cfg.Branch)" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  Next: open Claude Code in the worktree and execute the plan" -ForegroundColor White
    Write-Host ""
    Write-Host "    cd `"$wt`"" -ForegroundColor Gray
    Write-Host "    # Use superpowers:subagent-driven-development for parallel subagent execution" -ForegroundColor Gray
    Write-Host "    # Or superpowers:executing-plans for inline execution" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  When done:" -ForegroundColor White
    Write-Host "    cd `"$wt`"" -ForegroundColor Gray
    Write-Host "    git push -u origin $($cfg.Branch)" -ForegroundColor Gray
    Write-Host '    gh pr create --title "Path ' $key ': ..." --body "Closes N items from DESIGN stub-todo"' -ForegroundColor Gray
}

function Get-Status {
    Section "OpenForge Path Status"
    Write-Host ("  {0,-5} {1,-40} {2,-32} {3}" -f "Path", "Branch", "Worktree", "Status")
    Write-Host ("  {0,-5} {1,-40} {2,-32} {3}" -f "----", "------", "--------", "------")

    foreach ($key in @('A', 'B', 'C', 'D', 'X3', 'X4')) {
        $cfg = $Paths[$key]

        $branchExists = $false
        git rev-parse --verify "refs/heads/$($cfg.Branch)" *> $null
        $branchExists = ($LASTEXITCODE -eq 0)

        $wtExists = Test-Path $cfg.Dir
        $genExists = (Test-Path (Join-Path $cfg.Dir 'gen'))

        if ($branchExists -and $wtExists) {
            $status = if ($genExists) { "[OK] Ready (gen present)" } else { "[OK] Ready (run buf generate)" }
            $color = if ($genExists) { "Green" } else { "Yellow" }
        } elseif ($branchExists) {
            $status = "[!] Branch only"; $color = "Yellow"
        } elseif ($wtExists) {
            $status = "[!] Worktree only"; $color = "Yellow"
        } else {
            $status = "[X] Not started"; $color = "Gray"
        }
        Write-Host ("  {0,-5} {1,-40} {2,-32} {3}" -f $key, $cfg.Branch, $cfg.Dir, $status) -ForegroundColor $color
    }
}

function Test-PredMerged($key) {
    $predKey = $Pred[$key]
    if (-not $predKey) { return $true }
    $predCfg = $Paths[$predKey]
    git rev-parse --verify "refs/heads/$($predCfg.Branch)" *> $null
    return ($LASTEXITCODE -eq 0)
}

function Remove-Path($key) {
    if ($key -notin @('A','B','C','D','X3','X4')) {
        throw "clean: invalid path key '$key'"
    }
    Section "Removing Path $key"
    $cfg = $Paths[$key]
    if (Test-Path $cfg.Dir) {
        & git worktree remove --force $cfg.Dir
        Write-Host "[OK] removed worktree $($cfg.Dir)" -ForegroundColor Green
    }
    & git branch -D $cfg.Branch *> $null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[OK] deleted branch $($cfg.Branch)" -ForegroundColor Green
    }
}

# === MAIN ===
switch ($Action) {
    'setup' {
        Test-Prereqs
        foreach ($key in @('A','B','C','D','X3','X4')) { New-Wt $key }
        Write-Host ""
        Write-Host "[OK] All 6 path worktrees created. Next: launch.ps1 A" -ForegroundColor Green
    }
    'status' {
        Get-Status
    }
    {$_ -in @('A','B','C','D','X3','X4')} {
        Test-Prereqs
        if (-not (Test-PredMerged $Action)) {
            Write-Host "[X] Predecessor not ready. Check status first." -ForegroundColor Red
            exit 1
        }
        New-Wt $Action
        Test-Baseline $Action
        Show-Prompt $Action
    }
    'clean' {
        # Expect path key as second positional arg
        if ($args.Count -lt 1) {
            Write-Host "Usage: launch.ps1 clean <A|B|C|D|X3|X4>" -ForegroundColor Yellow
            exit 1
        }
        Remove-Path $args[0]
    }
    default {
        Write-Host "Usage: launch.ps1 {status|setup|clean <key>|A|B|C|D|X3|X4}" -ForegroundColor Yellow
        exit 1
    }
}
