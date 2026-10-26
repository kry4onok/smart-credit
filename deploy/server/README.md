# Deployment to 185.149.147.246

The site is served at https://185.149.147.246. The existing host runs Docker
Compose under `/opt/smart-credit`, with persistent database, document, JWT-key,
Kafka and Ollama volumes. The assistant uses Qwen2.5 7B on the host's CPU.

## GitHub Actions

`.github/workflows/deploy.yml` runs on pushes to `main` and can be started manually
from the Actions tab on `main`. Add the dedicated deployment private key as the
repository Actions secret `SERVER_SSH_KEY`. The host and its pinned public host
key are in the workflow; neither is a secret. Do not use a personal SSH key.

The matching server authorized key uses `restrict` and a forced command:
`/usr/bin/python3 /opt/smart-credit/deployer/receive.py`. It accepts only
`deploy <40-character commit SHA>` and a Git archive on stdin. It does not allow
interactive shells, port forwarding or arbitrary SSH commands.

The workflow must be present in `main` before automatic deployment can run.
Adding a repository secret alone does not enable the workflow.

## Release sequence

1. Receive an archive capped at 100 MiB and check its embedded Git revision.
2. Take an exclusive deployment lock and extract to a new release directory.
3. Apply the host's Qwen CPU compatibility patches, unless already in the source.
   A conflicting patch stops the deployment before changing running services.
4. Merge new non-secret `.env.example` defaults into the persistent server env.
   Existing values win. Missing existing database/account secrets, demo defaults,
   insecure cookies or a different model stop deployment.
5. Validate Compose, require at least 12 GiB free disk, build images and check
   Python imports as their non-root users.
6. Back up all three databases, check archive catalogs and apply forward migrations.
7. Update containers and wait for health checks. A short interruption is possible.
8. Verify HTTPS login, role, protected API, refresh-cookie recovery and logout for
   five demo accounts; confirm the installed Qwen model.
9. Publish the `current` symlink only after checks pass. Re-sending an unchanged
   successful revision is a no-op.

On failure after service switching, restore the previous service images. Database
migrations are **not** reversed automatically. Backwards-incompatible migrations
need a separate maintenance procedure; an image rollback cannot undo them.
Backups are local to this host and are not an offsite disaster-recovery solution.

## Persistent files and operations

- `/opt/smart-credit/shared/runtime.env`: server configuration and passwords,
  root-only mode 0600. Never replace it with `.env.example` on an update.
- `/opt/smart-credit/shared/access.txt`: credentials for the site, root-only.
- `/opt/smart-credit/shared/qwen-patches/`: compatibility patches for the existing
  CPU deployment. They are skipped when already incorporated into main.
- `/opt/smart-credit/current`: the last verified source release.
- `/opt/smart-credit/compose`: Compose wrapper for that release.
- `/opt/smart-credit/deploy-state.json`: current revision, backup path and status.
- `/opt/smart-credit/deploy-logs/`: protected deployment logs with known secrets redacted.
- `/opt/smart-credit/backups/`: database dumps made before each rollout.
- `/opt/smart-credit/deployer/`: installed copies of these deployment scripts and
  the server Compose overlay. Infrastructure/script changes are installed explicitly;
  application source is updated automatically from main.

The base three demo accounts are client, analyst and admin at `smart-credit.test`.
The `ip` and `closing` client accounts use the client password and the pre-existing
synthetic organizations selected by their INNs. Deployment does not reset passwords
or rerun the destructive parts of demonstration scenario initialization.

Inspect the server without printing secrets:

```sh
cat /opt/smart-credit/deploy-state.json
/opt/smart-credit/compose ps
python3 /opt/smart-credit/deployer/smoke.py
systemctl status smart-credit-certificate.timer
```

Run local archive-safety and permission tests:

```sh
python3 -m unittest discover -s deploy/server -p 'test_*.py' -v
```

Certificate renewal is handled independently by the existing
`smart-credit-certificate.timer`. SSH access, HTTPS certificates, database
passwords and model files survive application updates.
