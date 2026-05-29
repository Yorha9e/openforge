#!/bin/bash
# Database diagnostic script for OpenForge
# Run this script to diagnose database connection issues

echo "=== OpenForge Database Diagnostics ==="
echo "Timestamp: $(date)"
echo

# 1. Check PostgreSQL service status
echo "1. PostgreSQL Service Status:"
if command -v pg_isready &> /dev/null; then
    pg_isready -h localhost -p 5432
else
    echo "   pg_isready not found, checking with psql..."
    psql -h localhost -p 5432 -U openforge -d openforge -c "SELECT 1;" 2>&1 | head -5
fi
echo

# 2. Check PostgreSQL configuration
echo "2. PostgreSQL Configuration:"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT name, setting, unit, context 
FROM pg_settings 
WHERE name IN (
    'max_connections',
    'shared_buffers',
    'work_mem',
    'maintenance_work_mem',
    'effective_cache_size',
    'random_page_cost',
    'checkpoint_completion_target',
    'wal_buffers',
    'default_statistics_target'
)
ORDER BY name;"
echo

# 3. Check current connections
echo "3. Current Database Connections:"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT 
    count(*) as total_connections,
    count(*) FILTER (WHERE state = 'active') as active,
    count(*) FILTER (WHERE state = 'idle') as idle,
    count(*) FILTER (WHERE state = 'idle in transaction') as idle_in_transaction,
    count(*) FILTER (WHERE state = 'fastpath function call') as fastpath
FROM pg_stat_activity
WHERE datname = 'openforge';"
echo

# 4. Check connection details
echo "4. Connection Details:"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT 
    pid,
    usename,
    application_name,
    client_addr,
    backend_start,
    xact_start,
    query_start,
    state,
    LEFT(query, 100) as query_preview
FROM pg_stat_activity
WHERE datname = 'openforge'
ORDER BY backend_start DESC
LIMIT 20;"
echo

# 5. Check for long-running queries
echo "5. Long-running Queries (>30 seconds):"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT 
    pid,
    usename,
    NOW() - query_start as duration,
    state,
    LEFT(query, 200) as query_preview
FROM pg_stat_activity
WHERE datname = 'openforge'
    AND state = 'active'
    AND NOW() - query_start > INTERVAL '30 seconds'
ORDER BY duration DESC;"
echo

# 6. Check for idle in transaction connections
echo "6. Idle in Transaction Connections:"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT 
    pid,
    usename,
    NOW() - xact_start as xact_duration,
    NOW() - query_start as query_duration,
    state,
    LEFT(query, 200) as query_preview
FROM pg_stat_activity
WHERE datname = 'openforge'
    AND state = 'idle in transaction'
ORDER BY xact_duration DESC;"
echo

# 7. Check database size and table sizes
echo "7. Database and Table Sizes:"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT 
    pg_size_pretty(pg_database_size('openforge')) as database_size;
"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as index_size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;"
echo

# 8. Check for locks
echo "8. Current Locks:"
psql -h localhost -p 5432 -U openforge -d openforge -c "
SELECT 
    l.pid,
    l.mode,
    l.granted,
    a.usename,
    a.state,
    LEFT(a.query, 100) as query_preview
FROM pg_locks l
JOIN pg_stat_activity a ON l.pid = a.pid
WHERE l.relation IS NOT NULL
ORDER BY l.pid;"
echo

# 9. Check PostgreSQL logs for errors
echo "9. Recent PostgreSQL Errors (last 50 lines):"
if [ -f "/var/log/postgresql/postgresql-*.log" ]; then
    tail -50 /var/log/postgresql/postgresql-*.log | grep -i "error\|fatal\|panic" | tail -10
else
    echo "   PostgreSQL log file not found at default location."
    echo "   Check your PostgreSQL configuration for log file location."
fi
echo

# 10. Check system resources
echo "10. System Resources:"
echo "Memory:"
free -h 2>/dev/null || vm_stat 2>/dev/null || echo "   Memory info not available"
echo
echo "Disk Usage:"
df -h / 2>/dev/null || echo "   Disk info not available"
echo
echo "CPU Load:"
uptime 2>/dev/null || echo "   CPU load info not available"
echo

echo "=== Diagnostics Complete ==="
echo "If you see connection issues, consider:"
echo "1. Increasing max_connections in postgresql.conf"
echo "2. Adding connection timeout to database queries"
echo "3. Checking for connection leaks in application code"
echo "4. Monitoring for long-running queries"
