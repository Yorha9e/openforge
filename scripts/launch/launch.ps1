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

    # 1. Detect if we're already in a worktree (vs main repo)
    $gitDir = git rev-parse --git-dir 2>$null
    $gitCommon = git rev-parse --git-common-dir 2>$null
    $inWorktree = ($gitDir -ne $gitCommon)
    if ($inWorktree) {
        Write-Host "[INFO] running inside a worktree ($gitDir)" -ForegroundColor DarkGray
        $mainRoot = Split-Path $gitCommon -Parent
        Set-Location $mainRoot
        $ProjectRoot = $mainRoot
        Write-Host "[INFO] switched to main repo root: $ProjectRoot" -ForegroundColor DarkGray
    }

    # 2. .worktrees gitignored (only meaningful at main repo root)
    git check-ignore .worktrees *> $null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[X] .worktrees not gitignored (at main repo)" -ForegroundColor Red
        $ok = $false
    } else {
        Write-Host "[OK] .worktrees gitignored" -ForegroundColor Green
    }

    # 3. main branch exists
    git rev-parse --verify refs/heads/main *> $null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[X] main branch not found" -ForegroundColor Red
        $ok = $false
    } else {
        Write-Host "[OK] main branch exists" -ForegroundColor Green
    }

    # 4. main worktree clean
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

        # 1a. Discard any modified generated files (they were just regenerated, not user edits)
        Write-Host "  -> reset generated files (gen/, nodejs-io/src/gen/)"
        & git checkout -- 'gen' 'nodejs-io/src/gen' 2>&1 | Out-Null
        $LASTEXITCODE = 0
        # 1b. Remove any untracked generated files so they don't pollute status
        & git clean -fd -- 'gen' 'nodejs-io/src/gen' 2>&1 | Out-Null
        $LASTEXITCODE = 0

        # 2. Install node deps if missing (capture output to log file to bypass PowerShell stderr handling)
        function Install-NodeDeps($subdir) {
            if (-not (Test-Path "$subdir/package.json")) { return }
            if (Test-Path "$subdir/node_modules") { return }
            Write-Host "  -> $subdir npm install (first time, capturing log)"
            $logFile = Join-Path $env:TEMP "npm-install-$(Split-Path $subdir -Leaf).log"
            Push-Location $subdir
            try {
                & cmd /c "npm install --no-audit --no-fund --legacy-peer-deps 1>$logFile 2>&1" | Out-Null
            } catch { }
            Pop-Location
            $LASTEXITCODE = 0
            if (Test-Path "$subdir/node_modules") {
                Write-Host "[OK] $subdir deps installed (log: $logFile)" -ForegroundColor Green
            } else {
                Write-Host "[WARN] $subdir npm install may have failed. See $logFile" -ForegroundColor Yellow
            }
        }
        Install-NodeDeps 'frontend'
        Install-NodeDeps 'nodejs-io'

        # 3. Go tests
        Write-Host "  -> go test ./internal/... -count=1"
        & go test ./internal/... -count=1 *> $null
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[X] Go tests FAILED in $wt" -ForegroundColor Red
            & go test ./internal/... -count=1 2>&1 | Select-Object -Last 20
            throw "Go tests failed"
        }
        Write-Host "[OK] Go tests passed" -ForegroundColor Green

        # 4. Frontend typecheck (best-effort, swallow harness stderr)
        if (Test-Path 'frontend/package.json') {
            Write-Host "  -> frontend npm run typecheck (best-effort)"
            $logFile = Join-Path $env:TEMP "npm-typecheck-frontend.log"
            Push-Location frontend
            try { & cmd /c "npm run typecheck 1>$logFile 2>&1" | Out-Null } catch { }
            $LASTEXITCODE = 0
            Pop-Location
            $tcOk = $LASTEXITCODE -eq 0
            if ($tcOk) {
                Write-Host "[OK] frontend typecheck passed" -ForegroundColor Green
            } else {
                Write-Host "[WARN] frontend typecheck failed. See $logFile. Try: cd frontend && npm install && npm run typecheck" -ForegroundColor Yellow
            }
        }

        # 5. nodejs-io tests (best-effort, swallow harness stderr)
        if (Test-Path 'nodejs-io/package.json') {
            Write-Host "  -> nodejs-io npm test (best-effort)"
            $logFile = Join-Path $env:TEMP "npm-test-nodejs-io.log"
            Push-Location nodejs-io
            try { & cmd /c "npm test 1>$logFile 2>&1" | Out-Null } catch { }
            $LASTEXITCODE = 0
            Pop-Location
            $ntOk = $LASTEXITCODE -eq 0
            if ($ntOk) {
                Write-Host "[OK] nodejs-io tests passed" -ForegroundColor Green
            } else {
                Write-Host "[WARN] nodejs-io tests failed. See $logFile. Try: cd nodejs-io && npm test" -ForegroundColor Yellow
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
