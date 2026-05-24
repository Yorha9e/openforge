---
name: conduit-frontend
version: "1.0.0"
stages: [impl, test]
complexity: [L2, L3, L4]
permission: [auto, default]
keywords: [react, typescript, component, hooks, state, form, ui, frontend, conduit]
triggers:
  file_patterns: ["frontend/src/**/*.tsx", "*.tsx", "*.ts"]
  user_intent: [create page, add component, ui change, frontend change, fix frontend]
base_priority: 80
---

# Conduit Frontend Pattern

## Prompt
You are working on the Conduit RealWorld frontend, a React + TypeScript application.
Follow these conventions:
- Components are in `frontend/src/components/` or `frontend/src/features/`
- Use React Context + useReducer for state management
- Use react-hook-form + zod for forms
- API calls use a shared client in `frontend/src/api/`
- Routes use React Router in `frontend/src/routes/`
- Prefer editing existing files over creating new ones

## Workflow
1. Check existing components for reusable patterns
2. Read the API client for available endpoints
3. Create or modify components following existing conventions
4. Update routes if needed
