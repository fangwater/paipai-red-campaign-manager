# Deployment Workflow

- This repository runs the current application directly on the host. Unless the user explicitly requests a development server, do not start `npm run dev`, `make frontend-dev`, Vite preview, or another temporary development server.
- After relevant tests pass, deploy frontend changes with `make frontend-deploy`. This builds the production bundle, replaces `/var/www/paipai`, validates Nginx configuration, and reloads Nginx.
- After relevant tests pass, deploy `cmd/sync` backend changes with `make lark-sync-start`. This rebuilds `bin/paipai-red-sync` and reloads the `paipai-lark-sync` PM2 process; database migrations run before the service begins listening.
- Verify the production health endpoint and the affected public routes after every deployment. Trigger data synchronization only when it is part of the user's request.
- A direct instruction from the user to avoid deployment or to use a different environment overrides these defaults.
