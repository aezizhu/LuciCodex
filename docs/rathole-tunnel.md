# Rathole Tunnel Integration Guide

This guide explains how to expose your OpenWrt router's LuciCodex daemon to the internet using [Rathole](https://github.com/rapiz1/rathole), a lightweight reverse proxy.

## Why Rathole?

| Feature | Rathole | FRP | Cloudflared |
|---------|---------|-----|-------------|
| Binary Size | ~500KB-2MB | ~10MB+ | ~30MB+ |
| Memory Usage | ~5MB | ~20MB+ | ~30MB+ |
| Language | Rust | Go | Go |
| OpenWrt Support | Excellent | Good | Poor |

Rathole is ideal for resource-constrained routers with limited Flash and RAM.

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Your Device   │────▶│   VPS (Server)   │◀────│  OpenWrt Router │
│  (Web Browser)  │     │  Public IP       │     │  (Rathole Client)│
└─────────────────┘     └──────────────────┘     └─────────────────┘
        │                       │                        │
        │   HTTPS:443          │                        │
        └──────────────────────┤                        │
                               │   TCP:8443 (tunnel)    │
                               └────────────────────────┘
```

## Prerequisites

1. **VPS with public IP** - Any cheap VPS ($3-5/month) works
2. **Domain name** (optional but recommended for HTTPS)
3. **OpenWrt router** running LuciCodex

## Step 1: Server Setup (VPS)

### Install Rathole Server

```bash
# Download latest release (adjust version and architecture)
wget https://github.com/rapiz1/rathole/releases/download/v0.5.0/rathole-x86_64-unknown-linux-gnu.zip
unzip rathole-x86_64-unknown-linux-gnu.zip
chmod +x rathole
mv rathole /usr/local/bin/
```

### Create Server Configuration

```bash
cat > /etc/rathole/server.toml << 'EOF'
[server]
bind_addr = "0.0.0.0:8443"

# Optional: Enable TLS (recommended)
# [server.transport.tls]
# pkcs12 = "/path/to/identity.p12"
# pkcs12_password = "your-password"

[server.services.lucicodex]
token = "YOUR_SECRET_TOKEN_HERE"  # Generate with: openssl rand -hex 32
bind_addr = "127.0.0.1:8080"
EOF
```

### Create Systemd Service

```bash
cat > /etc/systemd/system/rathole-server.service << 'EOF'
[Unit]
Description=Rathole Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/rathole --server /etc/rathole/server.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now rathole-server
```

### Configure Nginx Reverse Proxy (Optional)

For HTTPS with Let's Encrypt:

```bash
cat > /etc/nginx/sites-available/lucicodex << 'EOF'
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400;  # For WebSocket
    }
}
EOF

ln -s /etc/nginx/sites-available/lucicodex /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

## Step 2: Router Setup (OpenWrt)

### Install Rathole Client

```bash
# For MIPS routers (most common)
wget https://github.com/rapiz1/rathole/releases/download/v0.5.0/rathole-mipsel-unknown-linux-musl.zip
unzip rathole-mipsel-unknown-linux-musl.zip
chmod +x rathole
mv rathole /usr/bin/

# For ARM routers
# wget https://github.com/rapiz1/rathole/releases/download/v0.5.0/rathole-arm-unknown-linux-musleabi.zip
```

### Create Client Configuration

```bash
cat > /etc/rathole.toml << 'EOF'
[client]
remote_addr = "YOUR_VPS_IP:8443"

# Optional: Enable TLS
# [client.transport.tls]
# trusted_root = "/etc/rathole/ca.crt"

[client.services.lucicodex]
token = "YOUR_SECRET_TOKEN_HERE"  # Must match server config
local_addr = "127.0.0.1:8888"     # LuciCodex daemon port
EOF
```

### Create Init Script

```bash
cat > /etc/init.d/rathole << 'EOF'
#!/bin/sh /etc/rc.common

START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/rathole --client /etc/rathole.toml
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
EOF

chmod +x /etc/init.d/rathole
/etc/init.d/rathole enable
/etc/init.d/rathole start
```

## Step 3: Configure LuciCodex Daemon

Ensure LuciCodex daemon is running on port 8888:

```bash
# In /etc/config/lucicodex or via LuCI
uci set lucicodex.config.daemon_enabled='1'
uci set lucicodex.config.daemon_port='8888'
uci commit lucicodex
/etc/init.d/lucicodex restart
```

## Step 4: Access from Anywhere

### Direct Access (via VPS)

```bash
# Health check
curl https://your-domain.com/health

# Generate a plan
curl -X POST https://your-domain.com/v1/plan \
  -H "Content-Type: application/json" \
  -H "X-Auth-Token: $(cat /tmp/.lucicodex.token)" \
  -d '{"prompt": "show network status"}'
```

### WebSocket Streaming

```javascript
const ws = new WebSocket('wss://your-domain.com/v1/ws?token=YOUR_TOKEN');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data);
};

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'chat',
    payload: { message: 'What is my WAN IP?' }
  }));
};
```

### MCP Client Integration

```json
{
  "mcpServers": {
    "lucicodex": {
      "url": "https://your-domain.com/v1/mcp",
      "headers": {
        "X-Auth-Token": "YOUR_TOKEN"
      }
    }
  }
}
```

## Security Considerations

1. **Use TLS** - Always enable TLS in production
2. **Strong Tokens** - Generate tokens with `openssl rand -hex 32`
3. **Firewall** - Only allow tunnel port (8443) on VPS
4. **Rate Limiting** - LuciCodex has built-in rate limiting (30 req burst, 2/sec)
5. **Auth Token** - Never expose the auth token publicly

## Troubleshooting

### Connection Refused

```bash
# Check if rathole is running
ps | grep rathole

# Check logs
logread | grep rathole
```

### Tunnel Not Establishing

```bash
# Test connectivity from router to VPS
nc -zv YOUR_VPS_IP 8443

# Check server logs
journalctl -u rathole-server -f
```

### WebSocket Timeout

Ensure Nginx has proper WebSocket headers and timeout settings:

```nginx
proxy_read_timeout 86400;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

## Alternative: SSH Tunnel (Simpler)

For quick testing without Rathole:

```bash
# On router
ssh -R 8080:127.0.0.1:8888 user@your-vps -N
```

This is simpler but less reliable for long-running connections.

## Resource Usage

| Component | Flash | RAM |
|-----------|-------|-----|
| Rathole Client | ~1MB | ~5MB |
| LuciCodex Daemon | ~7MB | ~20MB |
| **Total** | ~8MB | ~25MB |

Compatible with most routers having 16MB+ Flash and 64MB+ RAM.
