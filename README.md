# Soulmask Control

A modern, secure, and responsive web interface for managing Soulmask dedicated servers running in Docker containers. This tool provides real-time status monitoring, container lifecycle management, and live log streaming via WebSockets.

## 🚀 Features

- **Real-time Monitoring:** Instant visibility into your Soulmask server's status (Running, Stopped, etc.).
- **Container Management:** Direct controls to Start, Stop, and Restart the dedicated server container.
- **Live Logs:** Integrated WebSocket-based log streaming for real-time console monitoring.
- **Secure Authentication:** Simple yet effective password-protected access.
- **Auto-Update System:** Monitors the managed container's image for updates every 15 minutes. Automatically pulls new versions and recreates the container to apply patches.
- **Modern UI:** Built with Tailwind CSS, featuring a responsive design, interactive feedback, and a dedicated Update Monitor panel.
- **CI/CD Integrated:** Fully automated linting, security scanning (Gosec), and Docker image builds via GitHub Actions.

## 🛠️ Architecture

The application is split into a Go-based backend and a Vanilla JS/CSS frontend:

- **Backend (Go):**
  - Uses `gorilla/mux` for routing and `gorilla/websocket` for log streaming.
  - Interfaces directly with the Docker Engine API using the official Docker SDK.
  - Middleware handles session authentication and security headers.
- **Frontend (Static):**
  - **Tailwind CSS:** For a modern, utility-first UI design.
  - **Native WebSockets:** Efficient, low-latency log streaming without external dependencies.
  - **Local Assets:** All JS and CSS are hosted locally to ensure privacy and offline capability.

## 📦 Deployment

### Prerequisites
- Docker and Docker Compose
- A Soulmask dedicated server container running (or ready to be managed)

### Quick Start with Docker Compose

1. Clone the repository.
2. Configure your environment variables in `docker-compose.yml`.
3. Run:
   ```bash
   docker-compose up -d
   ```

### Configuration
Environment variables used by the server:
- `ADMIN_PASSWORD`: The password required for UI access.
- `TARGET_CONTAINER`: The name or ID of the Soulmask container to manage.
- `PORT`: (Optional) The port the web server listens on (default: 8080).
- `TRUST_PROXY`: Set to `true` if running behind a reverse proxy (enables Secure cookies).
- `DISCORD_WEBHOOK_URL`: (Optional) URL for Discord event notifications (server start/stop, container updates, etc.).

## 🛡️ Security

- **Gosec:** Automatic security scanning for common Go vulnerabilities.
- **Secure Cookies:** HttpOnly and SameSite attributes enforced.
- **Minimal Footprint:** Alpine-based Docker image for a reduced attack surface.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
