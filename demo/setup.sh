set -e
export GIT_AUTHOR_NAME=dev GIT_AUTHOR_EMAIL=dev@example.com GIT_COMMITTER_NAME=dev GIT_COMMITTER_EMAIL=dev@example.com
rm -rf /tmp/shop && mkdir -p /tmp/shop/src/api && cd /tmp/shop
git init -q -b main
cat > src/auth.ts <<'TS'
export function login(email: string): Session {
  return createSession(email);
}
TS
cat > src/api/profile.ts <<'TS'
import { login } from "../auth";

export const profile = (req: Request) => login(req.email);
TS
git add -A && git commit -qm "initial"

git checkout -qb agent/auth-hardening
cat > src/auth.ts <<'TS'
export function login(email: string, password: string): Session {
  verifyPassword(email, password);
  return createSession(email);
}
TS
cat > src/api/profile.ts <<'TS'
import { login } from "../auth";

export const profile = (req: Request) => login(req.email, req.password);
TS
git commit -qam "Require a password to log in"

git checkout -q main && git checkout -qb agent/checkout-api
cat > src/api/checkout.ts <<'TS'
import { login } from "../auth";

export function checkout(req: Request) {
  const session = login(req.email);
  return startPayment(session, req.cart);
}
TS
git add -A && git commit -qm "Add checkout endpoint"
git checkout -q main
