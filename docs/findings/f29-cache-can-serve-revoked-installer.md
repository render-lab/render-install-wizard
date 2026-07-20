# F29: The deployment cache can serve a revoked installer for one day

- Severity: Medium
- Category: Incident response
- Status: Risk

## Problem

The deployment route template applies `stale-while-revalidate=86400` to an executable shell installer. The route is not yet deployed from this repository, but once shipped it authorizes edge caches to serve stale content for up to 24 hours. After an emergency rollback or revocation, users can therefore continue receiving a compromised or broken bootstrap even though the origin has been corrected.

## Files affected

- `deploy/render-com/route-handler.ts:36-52`

## Proposed solution

Remove `stale-while-revalidate` for executable content or reduce it to an incident-response-safe duration, and use cache directives that force timely revalidation. Add automated CDN purge to both release and rollback procedures, followed by verification of the production response body and digest from representative edges. Test the route's cache headers and add a rollback drill that proves a revoked installer stops being served within the documented recovery objective.
