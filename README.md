# pgs - self host

A self hostable, static site hosting service using `ssh`.

## features

- Fully manage static sites using `ssh`
- Unlimited projects, created instantly upon upload
- Deploy using `rsync`, `sftp`, `sshfs`, or `scp`
- Github Action
- Automatic TLS for all projects
- Promotion and rollback support
- Custom domains for projects
- Custom Redirects & Rewrites
- Custom Headers
- SPA support
- [HTTP Caching (RFC-7234)](https://datatracker.ietf.org/doc/html/rfc7234)
- Image manipulation API
- Prometheus integration

> [!IMPORTANT]
> We provide a fully managed version of this service at
> [pgs.sh](https://pgs.sh).

## deps

- `docker`
- `caddy` (on-demand tls)
- `imgproxy` (image manipulation api, optional)

## setup

Create `.env` file:

```
# required
PGS_DOMAIN=pgs.test # this should be your custom domain
FS_STORAGE_DIR=./data/storage
DATABASE_URL=./data/pgs.sqlite3
PGS_PROTOCOL=http

# defaults
USE_IMGPROXY=0
STORAGE_ADAPTER=fs
PGS_WEB_PORT=3000
PGS_SSH_PORT=2222
PGS_PROTOCOL=https
PGS_CACHE_TTL=600s # time.ParseDuration
PGS_CACHE_CONTROL=max-age=600

# imgproxy
USE_IMGPROXY=1
IMGPROXY_URL=http://imgproxy:8080
IMGPROXY_ALLOWED_SOURCES=local://
IMGPROXY_KEY=6465616462656566 # deadbeef
IMGPROXY_SALT=6465616462656566 # deadbeef
```

## docker

This is the only recommended way to self-host `pgs`.

> [!IMPORTANT]
> We recommend using `docker-compose`: See our
> [docker-compose.yml](./docker-compose.yml) file.

```bash
docker run -d \
  --env-file=.env \
  -p 2222:2222 \
  -p 3000:3000 \
  -v $(pwd)/data:/app/data \
  ghcr.io/picosh/pgs:latest
```

## setup user account

Copy your public key:

```bash
cat ~/.ssh/id_ed25519.pub
# pubkey: zzz
```

Create your user account:

```sql
INSERT INTO app_users (name) VALUES ('erock') RETURNING id; -- id: 1
INSERT INTO public_keys (user_id, name, public_key) VALUES (1, 'main', 'zzz');
INSERT INTO feature_flags (user_id, name, expires_at) VALUES (1, 'plus', '2100-01-01');
```

## local dev

For local development you need to add host entries for each project. For
example, add entries to `/etc/hosts`:

```bash
0.0.0.0 pgs.test
0.0.0.0 erock-project.pgs.test
```

## usage

> [!IMPORTANT]
> For more in-depth usage, go to our managed service [docs](https://pico.sh/pgs)

```bash
rsync -e "ssh -p 2222" -rv ./public/ localhost:/project
```

```bash
curl http://erock-project.pgs.test:3000/
```
