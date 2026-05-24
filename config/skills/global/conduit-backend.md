---
name: conduit-backend
version: "1.0.0"
stages: [impl, test, deploy]
complexity: [L2, L3, L4]
permission: [auto, default]
keywords: [express, typescript, api, route, middleware, model, backend, conduit]
triggers:
  file_patterns: ["backend/src/**/*.ts", "*.ts"]
  user_intent: [add api, create route, add endpoint, backend change, fix backend]
base_priority: 80
---

# Conduit Backend Pattern

## Prompt
You are working on the Conduit RealWorld backend, an Express + TypeScript application.
Follow these conventions:
- Routes are defined in `backend/src/routes/` using Express Router
- Models use TypeORM or Prisma in `backend/src/models/`
- Middleware is in `backend/src/middleware/` (auth, validation, error)
- Use zod for request validation
- API responses follow the `{ status, data, errors }` envelope pattern
- All endpoints are prefixed with `/api`

## Workflow
1. Read existing route files to understand the API pattern
2. Read the Prisma schema or model definitions for data shape
3. Create or modify route handler following existing conventions
4. Add or update middleware if auth/validation needed
5. Write test in `backend/src/__tests__/`
