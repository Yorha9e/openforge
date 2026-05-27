package server

import (
	"encoding/json"
	"net/http"

	"openforge/internal/shared/profile"
)

// RunbookEntry represents a runbook item.
type RunbookEntry struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	LastUpdated string   `json:"last_updated"`
}

// RunbookDetail represents a full runbook document.
type RunbookDetail struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Format  string `json:"format"` // "markdown" or "yaml"
}

// Hardcoded runbook entries (future: load from YAML/MD files)
var runbookEntries = []RunbookEntry{
	{
		ID:          "incident-response",
		Title:       "Incident Response Playbook",
		Description: "Standard procedures for handling production incidents",
		Category:    "operations",
		Tags:        []string{"incident", "production", "emergency"},
		LastUpdated: "2026-05-27",
	},
	{
		ID:          "database-recovery",
		Title:       "Database Recovery Procedures",
		Description: "Steps for recovering from database failures",
		Category:    "database",
		Tags:        []string{"postgresql", "backup", "restore"},
		LastUpdated: "2026-05-27",
	},
	{
		ID:          "scaling-guide",
		Title:       "Horizontal Scaling Guide",
		Description: "How to scale OpenForge horizontally",
		Category:    "infrastructure",
		Tags:        []string{"kubernetes", "scaling", "performance"},
		LastUpdated: "2026-05-27",
	},
	{
		ID:          "security-audit",
		Title:       "Security Audit Checklist",
		Description: "Security audit procedures and checklist",
		Category:    "security",
		Tags:        []string{"security", "audit", "compliance"},
		LastUpdated: "2026-05-27",
	},
	{
		ID:          "deployment-checklist",
		Title:       "Production Deployment Checklist",
		Description: "Pre-deployment and post-deployment checks",
		Category:    "deployment",
		Tags:        []string{"deployment", "production", "checklist"},
		LastUpdated: "2026-05-27",
	},
}

// Hardcoded runbook content (future: load from files)
var runbookContent = map[string]RunbookDetail{
	"incident-response": {
		ID:    "incident-response",
		Title: "Incident Response Playbook",
		Format: "markdown",
		Content: "# Incident Response Playbook\n\n## 1. Initial Response\n- Acknowledge the alert within 5 minutes\n- Assess severity (P1-P4)\n- Create incident ticket\n\n## 2. Communication\n- Notify stakeholders via Slack #incidents channel\n- Update status page if customer-facing\n- Schedule status updates every 30 minutes\n\n## 3. Investigation\n- Check monitoring dashboards\n- Review recent deployments\n- Analyze error logs\n\n## 4. Resolution\n- Implement fix or rollback\n- Verify fix in production\n- Monitor for recurrence\n\n## 5. Post-Mortem\n- Schedule blameless post-mortem within 48 hours\n- Document lessons learned\n- Update runbook if needed",
	},
	"database-recovery": {
		ID:    "database-recovery",
		Title: "Database Recovery Procedures",
		Format: "markdown",
		Content: "# Database Recovery Procedures\n\n## Backup Types\n- **Full Backup**: Daily at 02:00 UTC\n- **WAL Archiving**: Continuous\n- **Point-in-Time Recovery**: Available for last 7 days\n\n## Recovery Steps\n\n### 1. Assess Damage\n```sql\nSELECT pg_is_in_recovery();\nSELECT now() - pg_postmaster_start_time() AS uptime;\n```\n\n### 2. Stop Application\n```bash\nkubectl scale deployment openforge --replicas=0\n```\n\n### 3. Restore from Backup\n```bash\n# List available backups\nls -la /backups/postgres/\n\n# Restore specific backup\npg_restore -d openforge /backups/postgres/backup_20260527.dump\n```\n\n### 4. Verify Data\n```sql\nSELECT COUNT(*) FROM audit_log;\nSELECT MAX(created_at) FROM pipeline;\n```\n\n### 5. Restart Application\n```bash\nkubectl scale deployment openforge --replicas=3\n```",
	},
	"scaling-guide": {
		ID:    "scaling-guide",
		Title: "Horizontal Scaling Guide",
		Format: "markdown",
		Content: "# Horizontal Scaling Guide\n\n## Current Architecture\n- Stateless API servers\n- PostgreSQL single-writer\n- Redis for caching\n- MinIO for object storage\n\n## Scaling Steps\n\n### 1. API Servers\n```bash\nkubectl scale deployment openforge --replicas=5\n```\n\n### 2. Database (Read Replicas)\n```yaml\n# values.yaml\npostgresql:\n  readReplicas:\n    enabled: true\n    count: 2\n```\n\n### 3. Redis Cluster\n```bash\nkubectl scale statefulset redis --replicas=3\n```\n\n## Monitoring\n- Watch CPU/memory usage\n- Monitor request latency\n- Check database connection pool",
	},
	"security-audit": {
		ID:    "security-audit",
		Title: "Security Audit Checklist",
		Format: "markdown",
		Content: "# Security Audit Checklist\n\n## Authentication\n- [ ] JWT tokens expire after 24h\n- [ ] Refresh tokens rotate on use\n- [ ] Passwords hashed with bcrypt\n- [ ] OIDC provider configured\n\n## Authorization\n- [ ] RBAC roles defined\n- [ ] Project isolation enforced\n- [ ] API endpoints protected\n\n## Data Protection\n- [ ] TLS enabled for all connections\n- [ ] Secrets stored in Vault\n- [ ] Database encrypted at rest\n- [ ] Audit logs enabled\n\n## Network Security\n- [ ] Firewall rules reviewed\n- [ ] Internal services not exposed\n- [ ] Rate limiting enabled\n\n## Compliance\n- [ ] GDPR data retention policy\n- [ ] Audit trail complete\n- [ ] Data export capability",
	},
	"deployment-checklist": {
		ID:    "deployment-checklist",
		Title: "Production Deployment Checklist",
		Format: "markdown",
		Content: "# Production Deployment Checklist\n\n## Pre-Deployment\n- [ ] All tests passing\n- [ ] Code review approved\n- [ ] Staging environment verified\n- [ ] Database migrations tested\n- [ ] Rollback plan documented\n\n## Deployment\n- [ ] Deploy to canary (10% traffic)\n- [ ] Monitor for 15 minutes\n- [ ] Gradual rollout to 100%\n- [ ] Verify health checks\n\n## Post-Deployment\n- [ ] Monitor error rates\n- [ ] Check performance metrics\n- [ ] Verify feature flags\n- [ ] Update deployment log\n\n## Rollback Procedure\n```bash\n# Quick rollback\nkubectl rollout undo deployment/openforge\n\n# Database rollback\npsql -d openforge -f rollback_001.sql\n```",
	},
}

// handleRunbookList returns a list of available runbooks.
func handleRunbookList(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		of.FeatureFlags.RLock()
		enabled := of.FeatureFlags.ProductionOps
		of.FeatureFlags.RUnlock()
		if !enabled {
			writeError(w, 404, "feature disabled")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(runbookEntries)
	}
}

// handleRunbookDetail returns the content of a specific runbook.
func handleRunbookDetail(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		of.FeatureFlags.RLock()
		enabled := of.FeatureFlags.ProductionOps
		of.FeatureFlags.RUnlock()
		if !enabled {
			writeError(w, 404, "feature disabled")
			return
		}

		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "runbook ID required")
			return
		}

		detail, exists := runbookContent[id]
		if !exists {
			writeError(w, http.StatusNotFound, "runbook not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}