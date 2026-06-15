# dof-admin deployment

The admin frontend is embedded into the Go binary by `//go:embed`, so the release package does not need a separate web directory.

## Linux systemd deployment

1. Download the matching Linux archive from the GitHub Actions artifact or GitHub Release.

   For most x86_64 servers, use:

   ```bash
   dof-admin-linux-amd64.tar.gz
   ```

2. Install files:

   ```bash
   sudo mkdir -p /opt/dof-admin /etc/dof-admin
   sudo tar -xzf dof-admin-linux-amd64.tar.gz -C /opt/dof-admin
   sudo cp /opt/dof-admin/deploy/dof-admin.env.example /etc/dof-admin/dof-admin.env
   sudo cp /opt/dof-admin/deploy/dof-admin.service /etc/systemd/system/dof-admin.service
   sudo chmod +x /opt/dof-admin/dof-admin
   sudo chmod 600 /etc/dof-admin/dof-admin.env
   ```

3. Edit `/etc/dof-admin/dof-admin.env` and set database, PVF, and optional NPK paths.

4. Start service:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now dof-admin
   sudo systemctl status dof-admin
   ```

5. Open:

   ```text
   http://server-ip:8080
   ```

## Manual run

```bash
cp deploy/dof-admin.env.example .env
set -a
. ./.env
set +a
./dof-admin
```
