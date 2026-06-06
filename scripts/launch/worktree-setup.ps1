# 一次性建好 6 路径 worktree
# 用法: powershell -ExecutionPolicy Bypass -File scripts/launch/worktree-setup.ps1

$ErrorActionPreference = 'Stop'
$ProjectRoot = git rev-parse --show-toplevel
Set-Location $ProjectRoot

$Paths = @(
    @{ Key='A';  Branch='feat/path-A-data-closure';         Dir='.worktrees/feature/path-A-data-closure' }
    @{ Key='B';  Branch='feat/path-B-security-ux-closure';   Dir='.worktrees/feature/path-B-security-ux' }
    @{ Key='C';  Branch='feat/path-C-architecture-honesty';  Dir='.worktrees/feature/path-C-architecture' }
    @{ Key='D';  Branch='feat/path-D-enterprise-landing';    Dir='.worktrees/feature/path-D-enterprise' }
    @{ Key='X3'; Branch='feat/X3-db-24-optimization';        Dir='.worktrees/feature/path-X3-db-24-optimization' }
    @{ Key='X4'; Branch='feat/X4-proto-fault-bench';         Dir='.worktrees/feature/path-X4-proto-fault-bench' }
)

# Pre-check
if (-not (git check-ignore -q .worktrees)) {
    Write-Error ".worktrees is not gitignored. Add to .gitignore first."
}

# 拉最新 main
git fetch origin main 2>$null

foreach ($p in $Paths) {
    $branchExists = git show-ref --verify --quiet "refs/heads/$($p.Branch)" 2>$null
    $wtExists = Test-Path $p.Dir
    if ($wtExists) {
        Write-Host "  ⏭  $($p.Key): worktree already exists at $($p.Dir)" -ForegroundColor Yellow
        continue
    }
    if ($branchExists) {
        Write-Host "  ↻  $($p.Key): creating worktree (branch exists)" -ForegroundColor Cyan
        git worktree add $p.Dir $p.Branch
    } else {
        Write-Host "  +  $($p.Key): creating worktree + branch" -ForegroundColor Green
        git worktree add $p.Dir -b $p.Branch main
    }
}

Write-Host ""
Write-Host "✓ All worktrees set up. Use scripts/launch/launch.ps1 -Action status to see state." -ForegroundColor Green
Write-Host "  Then: scripts/launch/launch.ps1 -Action A   to bootstrap Path A" -ForegroundColor Green
