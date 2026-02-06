# Implementation Report: Sandbox Mode & Agent Sync Stability

## 1. Objective
Resolved critical agent synchronization failures and implemented a robust "Sandbox Mode" with Root Assume capabilities for secure infrastructure testing and user support.

## 2. Issues Addressed
- **Backend Startup Panic**: Fixed duplicate route registration for `/api/agent-server/v1/users` that caused the container to crash on startup.
- **BFF JSON Parsing Crashes**: Implemented JSON fail-safes in the console BFF to handle upstream 502/HTML errors gracefully.
- **Demo/Sandbox Sync**: Transitioned from localStorage-based state to a server-side public binding endpoint (`/api/sandbox/binding`), ensuring all users see the same bound node.
- **Root Admin Visibility**: Centralized all sandbox management tools (Assume & Binding) into the `/panel/management` page's Root-only section.

## 3. Implementation Details

### A. Backend (`accounts.svc.plus`)
- **Integrated Route Registry**: Moved agent routes into the central `api` package and removed legacy registrations in `main.go`.
- **Root Assume Logic**: 
    - `POST /api/auth/admin/assume`: Signs a temporary token for `sandbox@svc.plus`.
    - `POST /api/auth/admin/assume/revert`: Stateless endpoint for revert logging.
- **Public Binding Access**: Added `GET /sandbox/binding` (readable by any authenticated user) to allow VLESS QR codes to find their bound node without admin privileges.
- **Hourly UUID Rotation**: Enforced hourly rotation for the sandbox user's ProxyUUID with automatic UI refreshes.

### B. Frontend (`console.svc.plus`)
- **Host-Only Assume mechanism**: 
    - The console BFF manages the `xc_session_root` cookie for identity recovery.
    - **Header Banner**: Real-time identification of assume state with a one-click "Exit Sandbox" button.
- **`RootAssumeSandboxPanel`**: Two-step confirmation UI for admins to switch identities.
- **`SandboxNodeBindingPanel`**: Robust server-side binding management with immediate UI feedback and cross-browser sync.

## 4. Key Artifacts & Commits
- **Backend Fix (Panic)**: `97b7d64d` (fix: Remove redundant agent routes and handlers from main.go)
- **Frontend Consolidation**: Integrated Root tools into `management.tsx` and updated `Header.tsx`.

## 5. Status
- **Development**: Complete
- **Build**: All projects "Green" (verified via `go build` and `npm run build`)
- **Deployment**: Verified stable on Cloud Run
